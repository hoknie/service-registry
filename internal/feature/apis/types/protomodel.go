package types

import (
	"regexp"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

	"svc-registry/pkg/ptr"
)

type comments map[string]string

func pathKey(path []int32) string {
	var b strings.Builder
	for _, p := range path {
		b.WriteByte(byte(p >> 8))
		b.WriteByte(byte(p))
	}
	return b.String()
}

func commentsOf(f *descriptorpb.FileDescriptorProto) comments {
	out := comments{}
	for _, l := range f.GetSourceCodeInfo().GetLocation() {
		if c := strings.TrimSpace(l.GetLeadingComments()); c != "" {
			out[pathKey(l.GetPath())] = c
		}
	}
	return out
}

func (c comments) at(path ...int32) *string { return ptr.NonEmpty(c[pathKey(path)]) }

func (p *Proto) referencedOf(declared map[string]string) map[string]string {
	out := map[string]string{}
	use := func(t string) {
		name := strings.TrimPrefix(t, ".")
		if path, ok := declared[name]; ok {
			out[name] = path
		}
	}
	for _, s := range p.Services {
		for _, m := range s.Methods {
			use(m.Input)
			use(m.Output)
		}
	}
	for _, m := range p.Messages {
		for _, f := range m.Fields {
			use(f.Type)
			if f.Map != nil {
				use(f.Map.Key)
				use(f.Map.Value)
			}
		}
	}
	return out
}

func protoModel(f, plain *descriptorpb.FileDescriptorProto) *Proto {
	out := emptyProto()
	out.Package = ptr.NonEmpty(f.GetPackage())
	out.Imports = append(out.Imports, f.GetDependency()...)
	for _, o := range plain.GetOptions().GetUninterpretedOption() {
		if len(o.GetName()) == 1 && !o.GetName()[0].GetIsExtension() && o.StringValue != nil {
			out.Options[o.GetName()[0].GetNamePart()] = string(o.GetStringValue())
		}
	}
	cm := commentsOf(f)
	prefix := ""
	if f.GetPackage() != "" {
		prefix = f.GetPackage() + "."
	}
	for si, s := range f.GetService() {
		sv := Service{Name: s.GetName(), FullName: prefix + s.GetName(), Description: cm.at(6, int32(si)), Methods: []Method{}}
		var plainMethods []*descriptorpb.MethodDescriptorProto
		if si < len(plain.GetService()) {
			plainMethods = plain.GetService()[si].GetMethod()
		}
		for mi, m := range s.GetMethod() {
			me := Method{Name: m.GetName(), Input: m.GetInputType(), Output: m.GetOutputType(),
				ClientStreaming: m.GetClientStreaming(), ServerStreaming: m.GetServerStreaming(),
				Description: cm.at(6, int32(si), 2, int32(mi)), Deprecated: m.GetOptions().GetDeprecated()}
			if mi < len(plainMethods) {
				opts := plainMethods[mi].GetOptions().GetUninterpretedOption()
				me.HTTP = httpRule(opts)
				me.Deprecated = me.Deprecated || boolOption(opts, "deprecated")
			}
			sv.Methods = append(sv.Methods, me)
		}
		out.Services = append(out.Services, sv)
	}
	for i, m := range f.GetMessageType() {
		out.message(m, prefix, cm, []int32{4, int32(i)}, f.GetSyntax())
	}
	for i, e := range f.GetEnumType() {
		out.Enums = append(out.Enums, enum(e, prefix, cm, []int32{5, int32(i)}))
	}
	return out
}

func (out *Proto) message(m *descriptorpb.DescriptorProto, prefix string, cm comments, path []int32, syntax string) {
	full := prefix + m.GetName()
	entries := map[string]*descriptorpb.DescriptorProto{}
	for _, n := range m.GetNestedType() {
		if n.GetOptions().GetMapEntry() {
			entries[n.GetName()] = n
		}
	}
	msg := Message{FullName: full, Description: cm.at(path...), Deprecated: m.GetOptions().GetDeprecated(), Fields: []Field{}}
	for fi, fd := range m.GetField() {
		field := Field{Name: fd.GetName(), Number: fd.GetNumber(), Type: fieldType(fd),
			Description: cm.at(append(append([]int32{}, path...), 2, int32(fi))...), Deprecated: fd.GetOptions().GetDeprecated()}
		name := fd.GetTypeName()
		if e, ok := entries[name[strings.LastIndex(name, ".")+1:]]; ok && fd.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			field.Map = &MapType{Key: fieldType(e.GetField()[0]), Value: fieldType(e.GetField()[1])}
		} else {
			field.Label = label(fd, syntax)
		}
		if fd.OneofIndex != nil && !fd.GetProto3Optional() && int(fd.GetOneofIndex()) < len(m.GetOneofDecl()) {
			field.Oneof = new(m.GetOneofDecl()[fd.GetOneofIndex()].GetName())
		}
		msg.Fields = append(msg.Fields, field)
	}
	out.Messages = append(out.Messages, msg)
	for i, n := range m.GetNestedType() {
		if !n.GetOptions().GetMapEntry() {
			out.message(n, full+".", cm, append(append([]int32{}, path...), 3, int32(i)), syntax)
		}
	}
	for i, e := range m.GetEnumType() {
		out.Enums = append(out.Enums, enum(e, full+".", cm, append(append([]int32{}, path...), 4, int32(i))))
	}
}

func enum(e *descriptorpb.EnumDescriptorProto, prefix string, cm comments, path []int32) Enum {
	out := Enum{FullName: prefix + e.GetName(), Description: cm.at(path...), Values: []EnumValue{}}
	for i, v := range e.GetValue() {
		out.Values = append(out.Values, EnumValue{Name: v.GetName(), Number: v.GetNumber(),
			Description: cm.at(append(append([]int32{}, path...), 2, int32(i))...)})
	}
	return out
}

func fieldType(fd *descriptorpb.FieldDescriptorProto) string {
	if fd.GetTypeName() != "" {
		return fd.GetTypeName()
	}
	return strings.ToLower(strings.TrimPrefix(fd.GetType().String(), "TYPE_"))
}

func label(fd *descriptorpb.FieldDescriptorProto, syntax string) *string {
	switch {
	case fd.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return new("repeated")
	case fd.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		return new("required")
	case fd.GetProto3Optional(), fd.Label != nil && (syntax == "" || syntax == "proto2"):
		return new("optional")
	}
	return nil
}

var httpVerb = regexp.MustCompile(`\b(get|put|post|delete|patch)\s*:\s*"([^"]*)"`)
var httpBody = regexp.MustCompile(`\bbody\s*:\s*"([^"]*)"`)

func httpRule(opts []*descriptorpb.UninterpretedOption) *HTTPRule {
	for _, o := range opts {
		parts := o.GetName()
		if len(parts) == 0 || !parts[0].GetIsExtension() || strings.TrimPrefix(parts[0].GetNamePart(), ".") != "google.api.http" {
			continue
		}
		m := httpVerb.FindStringSubmatch(o.GetAggregateValue())
		if m == nil {
			return nil
		}
		rule := &HTTPRule{Method: strings.ToUpper(m[1]), Path: m[2]}
		if b := httpBody.FindStringSubmatch(o.GetAggregateValue()); b != nil {
			rule.Body = new(b[1])
		}
		return rule
	}
	return nil
}

func boolOption(opts []*descriptorpb.UninterpretedOption, name string) bool {
	for _, o := range opts {
		if len(o.GetName()) == 1 && o.GetName()[0].GetNamePart() == name {
			return o.GetIdentifierValue() == "true"
		}
	}
	return false
}
