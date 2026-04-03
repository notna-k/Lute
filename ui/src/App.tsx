import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import Layout from './components/Layout';
import ProtectedRoute from './components/ProtectedRoute';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import UserWorkers from './pages/UserWorkers';
import PublicWorkers from './pages/PublicWorkers';
import WorkerMetrics from './pages/WorkerMetrics';
import ExecutionsHistory from './pages/ExecutionsHistory';
import JobDetail from './pages/JobDetail';

function App() {
  return (
    <AuthProvider>
      <Router>
        <Layout>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route
              path="/dashboard"
              element={
                <ProtectedRoute>
                  <Dashboard />
                </ProtectedRoute>
              }
            />
            <Route
              path="/workers/:id"
              element={
                <ProtectedRoute>
                  <WorkerMetrics />
                </ProtectedRoute>
              }
            />
            <Route
              path="/workers"
              element={
                <ProtectedRoute>
                  <UserWorkers />
                </ProtectedRoute>
              }
            />
            <Route
              path="/public-workers"
              element={
                <ProtectedRoute>
                  <PublicWorkers />
                </ProtectedRoute>
              }
            />
            <Route
              path="/jobs"
              element={
                <ProtectedRoute>
                  <ExecutionsHistory />
                </ProtectedRoute>
              }
            />
            <Route
              path="/jobs/:id"
              element={
                <ProtectedRoute>
                  <JobDetail />
                </ProtectedRoute>
              }
            />
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </Layout>
      </Router>
    </AuthProvider>
  );
}

export default App;
