package types

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/sourceinfo"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func emptyProto() *Proto {
	return &Proto{Imports: []string{}, Options: map[string]string{}, Services: []Service{}, Messages: []Message{}, Enums: []Enum{},
		TypeFiles: map[string]string{}}
}

func ParseProto(ctx context.Context, p, content string, files *Files) (doc Document, err error) {
	doc = Document{Path: p, Format: FormatProto, Problems: []Problem{}, Proto: emptyProto()}
	defer func() {
		if r := recover(); r != nil {
			doc.Proto = emptyProto()
			doc.Problems = append(doc.Problems, problem(CodeInvalidDocument, p, 0, fmt.Sprintf("the file cannot be read: %v", r)))
		}
	}()
	tree, perr := parser.Parse(p, strings.NewReader(content), reporter.NewHandler(nil))
	if perr != nil {
		doc.Problems = append(doc.Problems, protoProblem(p, perr))
		return doc, nil
	}
	res, perr := parser.ResultFromAST(tree, true, reporter.NewHandler(nil))
	if perr != nil {
		doc.Problems = append(doc.Problems, protoProblem(p, perr))
		return doc, nil
	}
	plain := res.FileDescriptorProto()
	plain.SourceCodeInfo = sourceinfo.GenerateSourceInfo(tree, nil)
	doc.SpecVersion = new(syntaxOf(plain))
	missing := map[string]bool{}
	for _, dep := range plain.GetDependency() {
		if _, ok := files.ResolveImport(dep); !ok && !strings.HasPrefix(dep, "google/protobuf/") {
			missing[dep] = true
		}
	}
	chosen := plain
	var typeFiles map[string]string
	if len(missing) == 0 {
		linked, types, lerr := compile(ctx, p, content, files, missing)
		switch {
		case lerr != nil && len(missing) == 0:
			doc.Problems = append(doc.Problems, protoProblem(p, lerr))
		case linked != nil:
			chosen, typeFiles = linked, types
		}
	}
	for _, dep := range slices.Sorted(maps.Keys(missing)) {
		doc.Problems = append(doc.Problems, problem(CodeImportNotFound, p, 0, fmt.Sprintf("import %q is not found in the snapshot", dep)))
	}
	doc.Proto = protoModel(chosen, plain)
	doc.Proto.TypeFiles = doc.Proto.referencedOf(typeFiles)
	if len(doc.Proto.Messages) > MaxItems {
		doc.Proto.Messages = doc.Proto.Messages[:MaxItems]
		doc.Problems = append(doc.Problems, problem(CodeTooLarge, p, 0, "more than 5000 messages; the first 5000 are shown"))
	}
	return doc, nil
}

func compile(ctx context.Context, p, content string, files *Files, missing map[string]bool) (*descriptorpb.FileDescriptorProto, map[string]string, error) {
	var readErr error
	paths := map[string]string{}
	accessor := func(name string) (io.ReadCloser, error) {
		if name == p {
			return io.NopCloser(strings.NewReader(content)), nil
		}
		resolved, ok := files.ResolveImport(name)
		if !ok {
			if !strings.HasPrefix(name, "google/protobuf/") {
				missing[name] = true
			}
			return nil, fs.ErrNotExist
		}
		c, found, err := files.Read(resolved)
		if err != nil {
			readErr = err
			return nil, err
		}
		if !found {
			missing[name] = true
			return nil, fs.ErrNotExist
		}
		paths[name] = resolved
		return io.NopCloser(strings.NewReader(c)), nil
	}
	c := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(&protocompile.SourceResolver{Accessor: accessor}),
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter:       reporter.NewReporter(nil, nil),
	}
	out, err := c.Compile(ctx, p)
	if readErr != nil {
		return nil, nil, readErr
	}
	if err != nil {
		if len(missing) > 0 {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	types := map[string]string{}
	importedTypes(out[0], paths, types, map[string]bool{})
	return protodesc.ToFileDescriptorProto(out[0]), types, nil
}

func importedTypes(f protoreflect.FileDescriptor, paths, out map[string]string, seen map[string]bool) {
	imports := f.Imports()
	for i := range imports.Len() {
		dep := imports.Get(i).FileDescriptor
		if seen[dep.Path()] {
			continue
		}
		seen[dep.Path()] = true
		if resolved, ok := paths[dep.Path()]; ok {
			declaredTypes(dep.Messages(), dep.Enums(), resolved, out)
		}
		importedTypes(dep, paths, out, seen)
	}
}

func declaredTypes(messages protoreflect.MessageDescriptors, enums protoreflect.EnumDescriptors, path string, out map[string]string) {
	for i := range enums.Len() {
		out[string(enums.Get(i).FullName())] = path
	}
	for i := range messages.Len() {
		m := messages.Get(i)
		out[string(m.FullName())] = path
		declaredTypes(m.Messages(), m.Enums(), path, out)
	}
}

func protoProblem(p string, err error) Problem {
	var pos reporter.ErrorWithPos
	if errors.As(err, &pos) {
		return problem(CodeInvalidDocument, p, pos.GetPosition().Line, pos.Unwrap().Error())
	}
	return problem(CodeInvalidDocument, p, 0, err.Error())
}

func syntaxOf(f *descriptorpb.FileDescriptorProto) string {
	switch f.GetSyntax() {
	case "", "proto2":
		return "proto2"
	case "editions":
		return "editions " + strings.TrimPrefix(f.GetEdition().String(), "EDITION_")
	}
	return f.GetSyntax()
}
