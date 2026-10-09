"use client";

import { Check, ChevronDown } from "lucide-react";
import { Select as S } from "radix-ui";
import { Children, isValidElement, type OptionHTMLAttributes, type ReactElement, type ReactNode } from "react";

import { cn } from "@/lib/cn";

const NONE = "__none__";

type Option = { value: string; label: ReactNode; disabled?: boolean };

function optionsOf(children: ReactNode): Option[] {
  const out: Option[] = [];
  Children.forEach(children, (child) => {
    if (!isValidElement(child)) return;
    const el = child as ReactElement<OptionHTMLAttributes<HTMLOptionElement> & { children?: ReactNode }>;
    if (el.type === "option") {
      out.push({ value: String(el.props.value ?? ""), label: el.props.children, disabled: el.props.disabled });
    } else if (el.props.children) {
      out.push(...optionsOf(el.props.children));
    }
  });
  return out;
}

type Props = {
  value: string;
  onChange: (e: { target: { value: string } }) => void;
  children: ReactNode;
  id?: string;
  disabled?: boolean;
  required?: boolean;
  placeholder?: ReactNode;
  className?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean | "true" | "false";
  "aria-label"?: string;
  name?: string;
};

export function Select({ value, onChange, children, id, disabled, required, placeholder, className, name, ...aria }: Props) {
  const options = optionsOf(children);
  const toRadix = (v: string) => (v === "" ? NONE : v);
  const current = options.find((o) => o.value === value);
  return (
    <S.Root
      value={current ? toRadix(value) : undefined}
      onValueChange={(v) => onChange({ target: { value: v === NONE ? "" : v } })}
      disabled={disabled}
      required={required}
      name={name}
    >
      <S.Trigger
        id={id}
        {...aria}
        className={cn(
          "motion-control flex h-9 w-full min-w-0 items-center justify-between gap-2 rounded-md border border-field bg-surface px-3 text-left text-base text-ink",
          "hover:border-ink-2 focus-visible:border-ring disabled:opacity-60 aria-invalid:border-danger data-[placeholder]:text-muted",
          className,
        )}
      >
        <span className="min-w-0 truncate">
          <S.Value placeholder={placeholder} />
        </span>
        <S.Icon asChild>
          <ChevronDown aria-hidden="true" className="size-4 shrink-0 text-muted" />
        </S.Icon>
      </S.Trigger>
      <S.Portal>
        <S.Content
          position="popper"
          sideOffset={6}
          collisionPadding={8}
          className="motion-pop z-50 max-h-[min(var(--radix-select-content-available-height),22rem)] min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-lg border border-line bg-surface shadow-pop"
        >
          <S.Viewport className="p-1">
            {options.map((o) => (
              <S.Item
                key={o.value || NONE}
                value={toRadix(o.value)}
                disabled={o.disabled}
                className="motion-control relative flex cursor-pointer items-center gap-2 rounded-md py-1.5 pr-8 pl-2.5 text-sm text-ink outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-surface-3 data-[state=checked]:font-medium"
              >
                <S.ItemText>{o.label}</S.ItemText>
                <S.ItemIndicator className="absolute right-2.5 inline-flex">
                  <Check aria-hidden="true" className="size-4 text-signal" />
                </S.ItemIndicator>
              </S.Item>
            ))}
          </S.Viewport>
        </S.Content>
      </S.Portal>
    </S.Root>
  );
}
