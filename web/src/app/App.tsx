import { Navigate, Route, Routes } from "react-router-dom";
import { AdminShell } from "../layouts/AdminShell/AdminShell";
import { PlaceholderPage } from "../pages/Placeholder/PlaceholderPage";
import { StudentExamPage } from "../pages/StudentExam/StudentExamPage";
import { adminRoutes } from "./routes";

export function App() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <PlaceholderPage
            description="后续接入平台管理员、租户管理员和教师登录态。"
            title="登录入口"
          />
        }
      />
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
