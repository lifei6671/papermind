import { Navigate, Route, Routes } from "react-router-dom";
import type { ReactNode } from "react";
import { AdminShell } from "../layouts/AdminShell/AdminShell";
import { PlatformLoginPage } from "../pages/Login/PlatformLoginPage";
import { ExamEntryPage } from "../pages/Exam/ExamEntryPage";
import { StudentExamPage } from "../pages/StudentExam/StudentExamPage";
import { TenantEntryPage } from "../pages/Tenant/TenantEntryPage";
import { useSession } from "../auth/session-context";
import { buildAdminRoutes, routeVisibleForRole } from "./routes";

export function App() {
  const { session } = useSession();
  const adminRoutes = buildAdminRoutes(session?.user, session?.selectedSpaceID);

  return (
    <Routes>
      <Route path="/login" element={<PlatformLoginPage />} />
      <Route path="/tenant-entry" element={<RequireSession allowTenantUser><TenantEntryPage /></RequireSession>} />
      <Route path="/exam-entry" element={<ExamEntryPage />} />
      <Route path="/student/exam" element={<StudentExamPage />} />
      <Route element={<RequireSession><AdminShell routes={adminRoutes} /></RequireSession>}>
        {adminRoutes.map((route) => (
          <Route
            element={
              routeRequiresRouteGuard(route.group) &&
              !routeVisibleForRole(route, session?.user.role, session?.profileSpaces ?? [])
                ? <Navigate to="/" replace />
                : route.element
            }
            key={route.path}
            path={route.path}
          />
        ))}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}

function routeRequiresRouteGuard(group: string) {
  return group === "platform" || group === "tenant" || group === "exam";
}

function RequireSession({ allowTenantUser = false, children }: { allowTenantUser?: boolean; children: ReactNode }) {
  const { session } = useSession();
  if (!session) {
    return <Navigate to="/login" replace />;
  }
  if (!allowTenantUser && session.user.role === "tenant_user") {
    return <Navigate to="/tenant-entry" replace />;
  }
  return children;
}
