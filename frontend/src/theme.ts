/**
 * Light or dark, remembered between runs.
 *
 * Light is the default. The stylesheet defines light on plain `:root` and dark
 * under `[data-theme="dark"]`, so an unset attribute is already the default
 * theme and there is no flash before the saved choice is read.
 */
export type Theme = "light" | "dark";

const KEY = "nanachi.theme";

/** Reads the saved choice, defaulting to light. */
export function loadTheme(): Theme {
  try {
    return localStorage.getItem(KEY) === "dark" ? "dark" : "light";
  } catch {
    // A webview with storage disabled must still render, so fall back rather
    // than letting this throw during startup.
    return "light";
  }
}

/** Applies a theme to the document and remembers it. */
export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === "dark") {
    root.setAttribute("data-theme", "dark");
  } else {
    root.removeAttribute("data-theme");
  }
  // Lets the webview paint form controls and scrollbars to match.
  root.style.colorScheme = theme;
  try {
    localStorage.setItem(KEY, theme);
  } catch {
    // Not being able to remember the choice is not a reason to refuse it.
  }
}
