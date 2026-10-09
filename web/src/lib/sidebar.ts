export const SIDEBAR_KEY = "svcr-sidebar";
export type SidebarState = "expanded" | "collapsed";

const railQuery = "(min-width: 1024px) and (max-width: 1279.98px)";

function isState(value: unknown): value is SidebarState {
  return value === "expanded" || value === "collapsed";
}

export function sidebarCollapsed(): boolean {
  const value = document.documentElement.dataset.sidebar;
  if (isState(value)) return value === "collapsed";
  return window.matchMedia(railQuery).matches;
}

export function saveSidebar(state: SidebarState): void {
  document.documentElement.dataset.sidebar = state;
  try {
    window.localStorage.setItem(SIDEBAR_KEY, state);
  } catch {
  }
  window.dispatchEvent(new Event(SIDEBAR_KEY));
}

export function watchSidebar(onChange: () => void): () => void {
  const query = window.matchMedia(railQuery);
  query.addEventListener("change", onChange);
  window.addEventListener(SIDEBAR_KEY, onChange);
  return () => {
    query.removeEventListener("change", onChange);
    window.removeEventListener(SIDEBAR_KEY, onChange);
  };
}

export const SIDEBAR_SCRIPT = `(function(){try{var s=localStorage.getItem(${JSON.stringify(
  SIDEBAR_KEY,
)});if(s==="expanded"||s==="collapsed")document.documentElement.dataset.sidebar=s}catch(e){}})()`;
