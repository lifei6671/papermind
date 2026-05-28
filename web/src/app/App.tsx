import { Navigate, Route, Routes } from "react-router-dom";
import type { ReactNode } from "react";
import { AdminShell } from "../layouts/AdminShell/AdminShell";
import { PlatformLoginPage } from "../pages/Login/PlatformLoginPage";
import { ExamEntryPage } from "../pages/Exam/ExamEntryPage";
import { StudentExamPage } from "../pages/StudentExam/StudentExamPage";
import { useSession } from "../auth/session-context";
import { buildAdminRoutes, routeVisibleForRole } from "./routes";

export function App() {
  const { session } = useSession();
  const adminRoutes = buildAdminRoutes(session?.user);

  return (
    <Routes>
      <Route path="/login" element={<PlatformLoginPage />} />
      <Route path="/exam-entry" element={<ExamEntryPage />} />
      <Route path="/student/exam" element={<StudentExamPage />} />
      <Route element={<RequireSession><AdminShell routes={adminRoutes} /></RequireSession>}>
        {adminRoutes.map((route) => (
          <Route
            element={
              route.group === "exam" && !routeVisibleForRole(route, session?.user.role, session?.profileSpaces ?? [])
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

function RequireSession({ children }: { children: ReactNode }) {
  const { session } = useSession();
  if (!session) {
    return <Navigate to="/login" replace />;
  }
  return children;
}
