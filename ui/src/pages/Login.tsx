import { FormEvent, useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Terminal } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';
import { Alert, Button, Card, Field, Input } from '@/components/ui';

const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const { user, signIn } = useAuth();

  const redirectTo =
    (location.state as { from?: { pathname?: string } } | null)?.from?.pathname ?? '/';

  useEffect(() => {
    if (user) navigate(redirectTo, { replace: true });
  }, [user, navigate, redirectTo]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await signIn(email.trim(), password);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Authentication failed.';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className='flex min-h-screen items-center justify-center bg-bg px-4 py-12'>
      <div className='w-full max-w-md'>
        <div className='mb-6 flex flex-col items-center gap-2 text-center'>
          <span className='flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-fg-onPrimary shadow-card'>
            <Terminal className='h-6 w-6' />
          </span>
          <h1 className='text-2xl font-bold tracking-tight text-fg'>
            Welcome to Lute
          </h1>
          <p className='text-sm text-fg-muted'>
            Sign in to manage your distributed workers.
          </p>
        </div>

        <Card className='p-6'>
          <form className='flex flex-col gap-4' onSubmit={handleSubmit}>
            {error && <Alert tone='danger'>{error}</Alert>}
            <Field label='Email' htmlFor='login-email' required>
              <Input
                id='login-email'
                type='email'
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete='email'
                required
                autoFocus
              />
            </Field>
            <Field label='Password' htmlFor='login-password' required>
              <Input
                id='login-password'
                type='password'
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete='current-password'
                required
              />
            </Field>
            <Button
              type='submit'
              loading={loading}
              size='lg'
              fullWidth
              disabled={!email || !password}
            >
              Sign in
            </Button>
          </form>
        </Card>
      </div>
    </div>
  );
};

export default Login;
