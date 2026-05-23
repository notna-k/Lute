import {
  BrowserRouter,
  Navigate,
  Outlet,
  Route,
  Routes,
  useLocation,
} from 'react-router-dom';
import { AuthProvider, AuthBridgeUpdater } from './contexts/AuthContext';
import { ThemeProvider } from './contexts/ThemeContext';
import { AppShell } from './components/layout/AppShell';
import { ErrorBoundary } from './components/ErrorBoundary';
import { useAuth } from './contexts/AuthContext';
import { Spinner } from './components/ui';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Workers from './pages/Workers';
import WorkerDetail from './pages/WorkerDetail';
import Executions from './pages/Executions';
import ExecutionDetail from './pages/ExecutionDetail';
import Settings from './pages/Settings';

/** Wraps protected pages: one stable <Routes> tree avoids / ↔ /login redirect loops. */
function AuthGuard() {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className='flex min-h-screen items-center justify-center'>
        <Spinner size={28} />
      </div>
    );
  }

  if (!user) {
    return <Navigate to='/login' replace state={{ from: location }} />;
  }

  return <Outlet />;
}

function AppShellLayout() {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

function AppRoutes() {
  return (
    <Routes>
      <Route path='/login' element={<Login />} />
      <Route element={<AuthGuard />}>
        <Route element={<AppShellLayout />}>
          <Route path='/' element={<Dashboard />} />
          <Route path='/dashboard' element={<Navigate to='/' replace />} />
          <Route path='/workers' element={<Workers />} />
          <Route path='/workers/:id' element={<WorkerDetail />} />
          <Route path='/executions' element={<Executions />} />
          <Route path='/executions/:id' element={<ExecutionDetail />} />
          <Route path='/settings' element={<Settings />} />
          <Route path='/jobs' element={<Navigate to='/executions' replace />} />
          <Route path='/jobs/:id' element={<LegacyJobRedirect />} />
          <Route path='*' element={<Navigate to='/' replace />} />
        </Route>
      </Route>
    </Routes>
  );
}

function LegacyJobRedirect() {
  const { pathname } = useLocation();
  const id = pathname.split('/').pop();
  return <Navigate to={`/executions/${id ?? ''}`} replace />;
}

function App() {
  return (
    <ThemeProvider>
      <ErrorBoundary>
        <AuthProvider>
          <AuthBridgeUpdater />
          <BrowserRouter>
            <AppRoutes />
          </BrowserRouter>
        </AuthProvider>
      </ErrorBoundary>
    </ThemeProvider>
  );
}

export default App;
