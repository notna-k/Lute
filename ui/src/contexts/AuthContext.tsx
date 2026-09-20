import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  ReactNode,
} from 'react';
import {
  AuthUser,
  login as apiLogin,
  logout as apiLogout,
  refresh as apiRefresh,
} from '../services/authService';

interface AuthContextType {
  user: AuthUser | null;
  loading: boolean;
  /** Latest access token (in memory only). null = not authenticated. */
  accessToken: string | null;
  signIn(email: string, password: string): Promise<void>;
  signOut(): Promise<void>;
  /** Try to silently refresh using the httpOnly cookie. Returns new access token or null. */
  refresh(): Promise<string | null>;
}

const AuthContext = createContext<AuthContextType | null>(null);

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider');
  return ctx;
};

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider = ({ children }: AuthProviderProps) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  // Single-flight: every concurrent caller of refresh() awaits the same promise.
  const inflight = useRef<Promise<string | null> | null>(null);

  const refresh = useCallback(async (): Promise<string | null> => {
    if (inflight.current) return inflight.current;
    inflight.current = (async () => {
      try {
        const res = await apiRefresh();
        setAccessToken(res.access_token);
        setUser(res.user);
        return res.access_token;
      } catch {
        setAccessToken(null);
        setUser(null);
        return null;
      } finally {
        inflight.current = null;
      }
    })();
    return inflight.current;
  }, []);

  const signIn = useCallback(async (email: string, password: string) => {
    const res = await apiLogin(email, password);
    setAccessToken(res.access_token);
    setUser(res.user);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      setAccessToken(null);
      setUser(null);
    }
  }, []);

  // On mount, try to silently restore a session using the refresh cookie.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      await refresh();
      if (!cancelled) setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [refresh]);

  const value = useMemo<AuthContextType>(
    () => ({ user, loading, accessToken, signIn, signOut, refresh }),
    [user, loading, accessToken, signIn, signOut, refresh],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

/**
 * Exported so non-React modules (e.g. the api client) can read the current token
 * and request a refresh. AuthProvider sets these on each render.
 */
export const authBridge: {
  getAccessToken: () => string | null;
  refresh: () => Promise<string | null>;
  signOut: () => Promise<void>;
} = {
  getAccessToken: () => null,
  refresh: async () => null,
  signOut: async () => {},
};

export const AuthBridgeUpdater = () => {
  const { accessToken, refresh, signOut } = useAuth();
  authBridge.getAccessToken = () => accessToken;
  authBridge.refresh = refresh;
  authBridge.signOut = signOut;
  return null;
};
