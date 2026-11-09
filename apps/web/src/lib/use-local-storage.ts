"use client";

import { useCallback, useSyncExternalStore } from "react";

const EVENT = "local-storage";

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

/** Write a localStorage value (null removes it) and notify every useLocalStorage hook. */
export function writeLocalStorage(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // storage blocked (private mode): hooks still re-read and fall back
  }
  window.dispatchEvent(new CustomEvent(EVENT, { detail: key }));
}

/** Reads a localStorage value and re-renders when it changes (this tab or another). Server: null. */
export function useLocalStorage(key: string): [string | null, (value: string | null) => void] {
  const subscribe = useCallback((onChange: () => void) => {
    const onStorage = (e: Event) => {
      const k = e instanceof StorageEvent ? e.key : (e as CustomEvent<string>).detail;
      if (k === key || k === null) onChange();
    };
    window.addEventListener("storage", onStorage);
    window.addEventListener(EVENT, onStorage);
    return () => {
      window.removeEventListener("storage", onStorage);
      window.removeEventListener(EVENT, onStorage);
    };
  }, [key]);
  const value = useSyncExternalStore(subscribe, () => read(key), () => null);
  const set = useCallback((v: string | null) => writeLocalStorage(key, v), [key]);
  return [value, set];
}
