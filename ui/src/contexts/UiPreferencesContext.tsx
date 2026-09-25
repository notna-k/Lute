import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

// Per-browser view preferences (rail width, row density); never sent to the server.

export type Density = 'roomy' | 'compact';

const SIDEBAR_KEY = 'lute.sidebar';
const DENSITY_KEY = 'lute.density';

interface UiPreferencesValue {
  sidebarExpanded: boolean;
  setSidebarExpanded: (expanded: boolean) => void;
  toggleSidebar: () => void;
  density: Density;
  setDensity: (density: Density) => void;
}

const UiPreferencesContext = createContext<UiPreferencesValue | null>(null);

function read(key: string, fallback: string): string {
  try {
    return window.localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

function write(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    /* storage unavailable (private window, blocked site data) */
  }
}

export function UiPreferencesProvider({ children }: { children: ReactNode }) {
  const [sidebarExpanded, setExpanded] = useState(() => read(SIDEBAR_KEY, 'rail') === 'expanded');
  const [density, setDensityState] = useState<Density>(() =>
    read(DENSITY_KEY, 'roomy') === 'compact' ? 'compact' : 'roomy',
  );

  // Density rides on the document element so the stylesheet can key off it.
  useEffect(() => {
    if (density === 'compact') {
      document.documentElement.dataset.density = 'compact';
    } else {
      delete document.documentElement.dataset.density;
    }
  }, [density]);

  const setSidebarExpanded = useCallback((expanded: boolean) => {
    setExpanded(expanded);
    write(SIDEBAR_KEY, expanded ? 'expanded' : 'rail');
  }, []);

  const toggleSidebar = useCallback(
    () =>
      setExpanded((prev) => {
        write(SIDEBAR_KEY, prev ? 'rail' : 'expanded');
        return !prev;
      }),
    [],
  );

  const setDensity = useCallback((next: Density) => {
    setDensityState(next);
    write(DENSITY_KEY, next);
  }, []);

  const value = useMemo(
    () => ({
      sidebarExpanded,
      setSidebarExpanded,
      toggleSidebar,
      density,
      setDensity,
    }),
    [sidebarExpanded, setSidebarExpanded, toggleSidebar, density, setDensity],
  );

  return <UiPreferencesContext.Provider value={value}>{children}</UiPreferencesContext.Provider>;
}

export function useUiPreferences(): UiPreferencesValue {
  const ctx = useContext(UiPreferencesContext);
  if (!ctx) {
    throw new Error('useUiPreferences must be used inside UiPreferencesProvider');
  }
  return ctx;
}
