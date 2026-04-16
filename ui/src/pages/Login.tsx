import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Terminal } from "lucide-react";
import { signInWithGoogle } from "@/services/authService";
import { useAuth } from "@/contexts/AuthContext";
import { Alert, Button, Card } from "@/components/ui";

interface FirebaseAuthError {
  code?: string;
  message?: string;
}

function resolveErrorMessage(err: unknown): string {
  const e = (err ?? {}) as FirebaseAuthError;
  if (e.code === "auth/popup-closed-by-user")
    return "Sign-in was cancelled. Please try again.";
  if (e.code === "auth/popup-blocked")
    return "Popup was blocked. Please allow popups and try again.";
  if (e.code === "auth/network-request-failed")
    return "Network error. Please check your connection.";
  if (e.message) return e.message;
  return "Authentication failed. Please try again.";
}

function GoogleMark() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" aria-hidden>
      <path
        fill="#EA4335"
        d="M12 10.2v3.9h5.45c-.24 1.27-1.6 3.74-5.45 3.74-3.28 0-5.96-2.72-5.96-6.08S8.72 5.68 12 5.68c1.87 0 3.12.8 3.84 1.48l2.62-2.53C16.83 3.14 14.63 2.2 12 2.2 6.9 2.2 2.8 6.3 2.8 11.4S6.9 20.6 12 20.6c6.93 0 9.36-4.86 9.36-7.56 0-.51-.05-.9-.13-1.28H12z"
      />
    </svg>
  );
}

const Login = () => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();
  const { user } = useAuth();

  useEffect(() => {
    if (user) navigate("/");
  }, [user, navigate]);

  const handleGoogleAuth = async () => {
    setLoading(true);
    setError(null);
    try {
      await signInWithGoogle();
    } catch (err) {
      console.error("Authentication error:", err);
      setError(resolveErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg px-4 py-12">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center gap-2 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-fg-onPrimary shadow-card">
            <Terminal className="h-6 w-6" />
          </span>
          <h1 className="text-2xl font-bold tracking-tight text-fg">
            Welcome to Lute
          </h1>
          <p className="text-sm text-fg-muted">
            Sign in to manage your distributed workers.
          </p>
        </div>

        <Card className="p-6">
          <div className="flex flex-col gap-4">
            {error && <Alert tone="danger">{error}</Alert>}
            <Button
              onClick={handleGoogleAuth}
              loading={loading}
              variant="secondary"
              size="lg"
              fullWidth
              leftIcon={<GoogleMark />}
            >
              Continue with Google
            </Button>
            <p className="text-center text-xs text-fg-muted">
              By continuing, you agree to our Terms of Service.
            </p>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default Login;
