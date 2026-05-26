import { Navigate, Route, Routes } from "react-router-dom";
import { AdminShell } from "../layouts/AdminShell/AdminShell";
import { PlatformLoginPage } from "../pages/Login/PlatformLoginPage";
import { StudentExamPage } from "../pages/StudentExam/StudentExamPage";
import { adminRoutes } from "./routes";

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<PlatformLoginPage />} />
      <Route path="/student/exam" element={<StudentExamPage />} />
      <Route element={<AdminShell routes={adminRoutes} />}>
        {adminRoutes.map((route) => (
          <Route key={route.path} path={route.path} element={route.element} />
        ))}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}
