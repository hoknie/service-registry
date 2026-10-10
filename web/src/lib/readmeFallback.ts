export function isRootReadme(path: string): boolean {
  if (path.includes("/")) return false;
  const name = path.toLowerCase();
  return name === "readme" || (name.startsWith("readme.") && name.length > "readme.".length);
}

export function pickReadme<T extends { path: string }>(files: T[]): T | null {
  const found = files.filter((f) => isRootReadme(f.path));
  if (!found.length) return null;
  return found.find((f) => f.path.toLowerCase() === "readme.md") ?? [...found].sort((a, b) => a.path.localeCompare(b.path))[0]!;
}

export function isMarkdownReadme(path: string): boolean {
  const name = path.toLowerCase();
  return name === "readme" || name.endsWith(".md") || name.endsWith(".markdown");
}
