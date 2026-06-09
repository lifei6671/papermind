import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { lazy, Suspense, useEffect, type ReactElement, type ReactNode } from "react";
import { AdminShell } from "../layouts/AdminShell/AdminShell";
import { useSession } from "../auth/session-context";
import { pageTitleForPathname } from "./page-title";
import { buildAdminRoutes, routeVisibleForRole } from "./routes";

const PlatformLoginPage = lazy(() => import("../pages/Login/PlatformLoginPage").then(({ PlatformLoginPage }) => ({ default: PlatformLoginPage })));
const ExamEntryPage = lazy(() => import("../pages/Exam/ExamEntryPage").then(({ ExamEntryPage }) => ({ default: ExamEntryPage })));
const PaperStudentPreviewRoute = lazy(() => import("../pages/Exam/PaperPreviewRoute").then(({ PaperStudentPreviewRoute }) => ({ default: PaperStudentPreviewRoute })));
const StudentExamPage = lazy(() => import("../pages/StudentExam/StudentExamPage").then(({ StudentExamPage }) => ({ default: StudentExamPage })));
const TenantEntryPage = lazy(() => import("../pages/Tenant/TenantEntryPage").then(({ TenantEntryPage }) => ({ default: TenantEntryPage })));

export function App() {
  const { session } = useSession();
  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const scopedSpaceID = scopedSpaceIDFromSession(session, searchParams);
  const scopedExamID = positiveID(searchParams.get("exam_id"));
  const adminRoutes = buildAdminRoutes(session?.user, scopedSpaceID, session?.profileSpaces ?? [], scopedExamID);

  useEffect(() => {
    document.title = pageTitleForPathname(location.pathname, adminRoutes);
  }, [location.pathname, adminRoutes]);

  return (
    <Routes>
      <Route path="/login" element={withRouteSuspense(<PlatformLoginPage />)} />
      <Route path="/tenant-entry" element={<RequireSession allowTenantUser>{withRouteSuspense(<TenantEntryPage />)}</RequireSession>} />
      <Route path="/exam-entry" element={withRouteSuspense(<ExamEntryPage />)} />
      <Route path="/student/exam" element={withRouteSuspense(<StudentExamPage />)} />
      <Route
        path="/papers/:paperID/student-preview"
        element={(
          <RequireSession>
            {withRouteSuspense(<PaperStudentPreviewRoute tenantID={session?.user.tenantID ?? 0} spaceID={scopedSpaceID} />)}
          </RequireSession>
        )}
      />
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
                : withRouteSuspense(route.element)
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

function withRouteSuspense(element: ReactElement) {
  return (
    <Suspense fallback={<RouteLoadingFallback />}>
      {element}
    </Suspense>
  );
}

function RouteLoadingFallback() {
  return <div className="tenant-admin-status" role="status">正在加载页面</div>;
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
