import { Navigate, Route, Routes, useLocation } from "react-router-dom";
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
  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const scopedSpaceID = scopedSpaceIDFromSession(session, searchParams);
  const scopedExamID = positiveID(searchParams.get("exam_id"));
  const adminRoutes = buildAdminRoutes(session?.user, scopedSpaceID, session?.profileSpaces ?? [], scopedExamID);

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
              routeRequiresRouteGuard(route) &&
              !routeVisibleForRole(route, session?.user.role, session?.profileSpaces ?? [], {
                tenantID: session?.user.tenantID,
                spaceID: scopedSpaceID,
              })
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

function positiveID(value: string | null) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

function scopedSpaceIDFromSession(
  session: ReturnType<typeof useSession>["session"],
  searchParams: URLSearchParams,
) {
  if (!session || session.user.role === "platform_admin" || session.user.role === "tenant_admin") {
    return undefined;
  }
  return session.selectedSpaceID ?? positiveID(searchParams.get("space_id"));
}

function routeRequiresRouteGuard(route: { group: string; menuRoles?: string[]; spaceMemberRoles?: unknown[] }) {
  return route.group === "platform" ||
    route.group === "tenant" ||
    route.group === "exam" ||
    (route.group === "hidden" && (!!route.menuRoles || !!route.spaceMemberRoles));
}

function RequireSession({ allowTenantUser = false, children }: { allowTenantUser?: boolean; children: ReactNode }) {
  const { session } = useSession();
  const location = useLocation();
  if (!session) {
    return <Navigate to="/login" replace />;
  }
  if (session.user.forcePasswordChange && location.pathname !== "/settings/profile") {
    return <Navigate to="/settings/profile" replace />;
  }
  if (!allowTenantUser && session.user.role === "tenant_user" && location.pathname !== "/settings/profile") {
    return <Navigate to="/tenant-entry" replace />;
  }
  return children;
}
