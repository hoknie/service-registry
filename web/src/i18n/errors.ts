import type { Messages } from "./messages";

type Tree = { [key: string]: string | Tree };

export function errorText(errors: Messages["errors"], code: string): string {
  let node: string | Tree | undefined = errors as Tree;
  for (const part of code.split(".")) {
    node = typeof node === "object" ? node[part] : undefined;
  }
  return typeof node === "string" ? node : errors.unknown;
}
