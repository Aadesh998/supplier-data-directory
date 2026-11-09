// Light / dark / auto theme. The choice lives in localStorage; "auto" follows the OS.
// THEME_INIT_SCRIPT runs in <head> before first paint so the page never flashes the wrong theme.

export type ThemePreference = "light" | "dark" | "auto";

export const THEME_STORAGE_KEY = "theme";

export const THEME_INIT_SCRIPT = `(function(){try{var p=localStorage.getItem("${THEME_STORAGE_KEY}");var d=p==="dark"||((p!=="light")&&matchMedia("(prefers-color-scheme: dark)").matches);document.documentElement.classList.toggle("dark",d)}catch(e){}})();`;

export function toPreference(stored: string | null): ThemePreference {
  return stored === "light" || stored === "dark" ? stored : "auto";
}

/** Sets the "dark" class on <html> for a preference (storage is written by the caller). */
export function applyPreference(p: ThemePreference) {
  const dark = p === "dark" || (p === "auto" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
}
