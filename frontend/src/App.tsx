import { useEffect } from 'react';
import { createBrowserRouter, RouterProvider, Navigate, Outlet } from 'react-router-dom';
import { Agents } from '@/pages/Agents';
import { LoginPage } from '@/pages/LoginPage';
import { GoogleAuthCallbackPage } from '@/pages/GoogleAuthCallbackPage';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { AdminLayout } from '@/components/admin/AdminLayout';
import { ProviderManagement } from '@/pages/admin/ProviderManagement';
import { ModelManagement } from '@/pages/admin/ModelManagement';
import { useAuthStore } from '@/store/useAuthStore';

/** Guard: user must be logged-in AND have is_admin=true */
function AdminRoute() {
  const user = useAuthStore(s => s.user);
  const isAuthenticated = useAuthStore(s => s.isAuthenticated);

  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (!user?.is_admin) return <Navigate to="/agents" replace />;
  return <Outlet />;
}

const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    path: '/agents',
    element: <ProtectedRoute><Agents /></ProtectedRoute>,
  },
  {
    path: '/agents/builder/:agentId',
    element: <ProtectedRoute><Agents /></ProtectedRoute>,
  },
  {
    path: '/agents/deployed/:agentId',
    element: <ProtectedRoute><Agents /></ProtectedRoute>,
  },
  {
    path: '/admin',
    element: <AdminRoute />,
    children: [
      {
        element: <AdminLayout />,
        children: [
          { index: true, element: <Navigate to="/admin/providers" replace /> },
          { path: 'providers', element: <ProviderManagement /> },
          { path: 'models', element: <ModelManagement /> },
        ],
      },
    ],
  },
  { path: '/auth/google/callback', element: <GoogleAuthCallbackPage /> },
  { path: '/', element: <Navigate to="/agents" replace /> },
  { path: '*', element: <Navigate to="/agents" replace /> },
]);

export function App() {
  const initialize = useAuthStore(s => s.initialize);

  useEffect(() => {
    initialize();
  }, [initialize]);

  return <RouterProvider router={router} />;
}
