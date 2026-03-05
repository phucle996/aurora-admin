import { Suspense, lazy, type ReactNode } from "react";
import { Navigate, Outlet, Route, Routes } from "react-router-dom";

import AdminLayout from "@/layout/admin-layout";
import { Toaster } from "@/components/ui/sonner";
import { hasAdminSession } from "@/lib/admin-auth";
import {
  useEnabledModules,
  type ModuleFeature,
} from "@/state/enabled-modules-context";

import "./App.css";

const LoginPage = lazy(() => import("@/pages/AuthPage/Login"));
const DashboardPage = lazy(() => import("@/pages/DashboardPage/DashboardPage"));
const KvmPage = lazy(() => import("@/pages/HypervisorPage/KvmPage/KvmPage"));
const KvmDetailPage = lazy(() => import("@/pages/HypervisorPage/KvmDetailPage/KvmDetailPage"));
const NewKvmPage = lazy(() => import("@/pages/HypervisorPage/NewKvmPage/NewKvmPage"));
const DockerPage = lazy(() => import("@/pages/ContainerPage/DockerPage"));
const K8sPage = lazy(() => import("@/pages/K8sPage/K8sPage"));
const ModulePage = lazy(() => import("@/pages/ModulePage/ModulePage"));
const AdminSettingsPage = lazy(() => import("@/pages/SettingsPage/AdminSettingsPage"));
const NotFoundPage = lazy(() => import("@/pages/NotFoundPage/NotFoundPage"));

function RequireAdminSession() {
  if (!hasAdminSession()) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}

function RedirectRoot() {
  return <Navigate to={hasAdminSession() ? "/dashboard" : "/login"} replace />;
}

function LoginRoute() {
  if (hasAdminSession()) {
    return <Navigate to="/dashboard" replace />;
  }
  return <LoginPage />;
}

function RequireEnabledFeature({
  feature,
  children,
}: {
  feature: ModuleFeature;
  children: ReactNode;
}) {
  const { items, status, isFeatureEnabled } = useEnabledModules();

  if (status === "idle" || status === "loading") {
    if (items.length === 0) {
      return <RouteFallback />;
    }
    return <>{children}</>;
  }

  if (!isFeatureEnabled(feature)) {
    return <Navigate to="/module" replace />;
  }

  return <>{children}</>;
}

function App() {
  return (
    <>
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/" element={<RedirectRoot />} />
          <Route path="/login" element={<LoginRoute />} />
          <Route element={<RequireAdminSession />}>
            <Route element={<AdminLayout />}>
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route
                path="/hypervisor/kvm"
                element={
                  <RequireEnabledFeature feature="kvm">
                    <Outlet />
                  </RequireEnabledFeature>
                }
              >
                <Route index element={<KvmPage />} />
                <Route path="new" element={<NewKvmPage />} />
                <Route path=":nodeId/*" element={<KvmDetailPage />} />
              </Route>
              <Route
                path="/containers/docker"
                element={
                  <RequireEnabledFeature feature="docker">
                    <DockerPage />
                  </RequireEnabledFeature>
                }
              />
              <Route
                path="/orchestration/k8s"
                element={
                  <RequireEnabledFeature feature="k8s">
                    <K8sPage />
                  </RequireEnabledFeature>
                }
              />
              <Route path="/module" element={<ModulePage />} />
              <Route path="/settings" element={<AdminSettingsPage />} />
              <Route
                path="/vms"
                element={
                  <RequireEnabledFeature feature="kvm">
                    <Navigate to="/hypervisor/kvm" replace />
                  </RequireEnabledFeature>
                }
              />
            </Route>
          </Route>
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </Suspense>
      <Toaster />
    </>
  );
}

function RouteFallback() {
  return (
    <div className="grid min-h-screen place-items-center text-sm text-slate-500 dark:text-slate-300">
      Loading...
    </div>
  );
}

export default App;
