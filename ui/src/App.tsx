import { BrowserRouter, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AuthProvider } from "./contexts/AuthContext";
import { ThemeProvider } from "./contexts/ThemeContext";
import { AppShell } from "./components/layout/AppShell";
import { ErrorBoundary } from "./components/ErrorBoundary";
import ProtectedRoute from "./components/ProtectedRoute";
import Login from "./pages/Login";
import Dashboard from "./pages/Dashboard";
import Workers from "./pages/Workers";
import WorkerDetail from "./pages/WorkerDetail";
import Executions from "./pages/Executions";
import ExecutionDetail from "./pages/ExecutionDetail";

function AppRoutes() {
  const { pathname } = useLocation();
  const isAuthRoute = pathname === "/login";

  if (isAuthRoute) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
      </Routes>
    );
  }

  return (
    <AppShell>
      <Routes>
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          }
        />
        <Route path="/dashboard" element={<Navigate to="/" replace />} />
        <Route
          path="/workers"
          element={
            <ProtectedRoute>
              <Workers />
            </ProtectedRoute>
          }
        />
        <Route
          path="/workers/:id"
          element={
            <ProtectedRoute>
              <WorkerDetail />
            </ProtectedRoute>
          }
        />
        <Route
          path="/executions"
          element={
            <ProtectedRoute>
              <Executions />
            </ProtectedRoute>
          }
        />
        <Route
          path="/executions/:id"
          element={
            <ProtectedRoute>
              <ExecutionDetail />
            </ProtectedRoute>
          }
        />
        <Route path="/jobs" element={<Navigate to="/executions" replace />} />
        <Route
          path="/jobs/:id"
          element={<LegacyJobRedirect />}
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}

function LegacyJobRedirect() {
  const { pathname } = useLocation();
  const id = pathname.split("/").pop();
  return <Navigate to={`/executions/${id ?? ""}`} replace />;
}

function App() {
  return (
    <ThemeProvider>
      <ErrorBoundary>
        <AuthProvider>
          <BrowserRouter>
            <AppRoutes />
          </BrowserRouter>
        </AuthProvider>
      </ErrorBoundary>
    </ThemeProvider>
  );
}

export default App;
