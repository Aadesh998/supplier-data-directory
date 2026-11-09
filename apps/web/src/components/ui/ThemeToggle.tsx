"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useEffect } from "react";
import { applyPreference, THEME_STORAGE_KEY, toPreference, type ThemePreference } from "@/lib/theme";
import { useLocalStorage } from "@/lib/use-local-storage";

const NEXT: Record<ThemePreference, ThemePreference> = { light: "dark", dark: "auto", auto: "light" };
const LABEL: Record<ThemePreference, string> = { light: "Light", dark: "Dark", auto: "Auto" };
const ICON = { light: Sun, dark: Moon, auto: Monitor };

function readStored() {
  try {
    return localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    return null;
  }
}

/** Cycles light → dark → auto. `showLabel` renders the current mode next to the icon. */
export function ThemeToggle({ showLabel = false, className = "" }: { showLabel?: boolean; className?: string }) {
  const [stored, setStored] = useLocalStorage(THEME_STORAGE_KEY);
  const pref = toPreference(stored);

  // Keep <html class="dark"> in sync with the preference, and follow the OS live in auto mode.
  useEffect(() => {
    // Read storage directly: during hydration `pref` is briefly the server value ("auto").
    const current = toPreference(readStored());
    applyPreference(current);
    if (current !== "auto") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyPreference("auto");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [pref]);

  const Icon = ICON[pref];
  const next = NEXT[pref];
  return (
    <button
      type="button"
      className={`btn-ghost btn-sm ${className}`}
      onClick={() => setStored(next === "auto" ? null : next)}
      title={`Theme: ${LABEL[pref]} (switch to ${LABEL[next]})`}
      aria-label={`Theme: ${LABEL[pref]}. Switch to ${LABEL[next]}`}
    >
      <Icon aria-hidden="true" className="size-4 shrink-0" />
      {showLabel && <span>{LABEL[pref]}</span>}
    </button>
  );
}
