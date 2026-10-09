import { defaultLocale, type Locale } from "./config";
import en from "./messages/en.json";
import es from "./messages/es.json";
import ru from "./messages/ru.json";
import zh from "./messages/zh.json";

export type Messages = typeof en;

type Tree = { [key: string]: string | Tree };

const all: Record<Locale, Tree> = { en, es, ru, zh };

function overlay(base: Tree, over: Tree): Tree {
  const out: Tree = {};
  for (const [key, value] of Object.entries(base)) {
    const own = over[key];
    if (typeof value === "string") out[key] = typeof own === "string" && own ? own : value;
    else out[key] = overlay(value, typeof own === "object" ? own : {});
  }
  return out;
}

export function getMessages(locale: Locale): Messages {
  if (locale === defaultLocale) return en;
  return overlay(en, all[locale]) as Messages;
}
