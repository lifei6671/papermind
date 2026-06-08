import type { AdminRoute } from "./routes";

const APP_TITLE = "PaperMind";

export function pageTitleForPathname(pathname: string, routes: AdminRoute[]) {
  const pageTitle = standalonePageTitle(pathname) ?? routes.find((route) => routePathMatches(route.path, pathname))?.label;
  return pageTitle ? `${pageTitle} - ${APP_TITLE}` : APP_TITLE;
}

function standalonePageTitle(pathname: string) {
  if (routePathMatches("/login", pathname)) {
    return "平台管理员登录";
  }
  if (routePathMatches("/tenant-entry", pathname)) {
    return "选择租户";
  }
  if (routePathMatches("/exam-entry", pathname)) {
    return "考试入口";
  }
  if (routePathMatches("/student/exam", pathname)) {
    return "在线考试";
  }
  if (routePathMatches("/papers/:paperID/student-preview", pathname)) {
    return "学生视角预览";
  }
  return undefined;
}

function routePathMatches(pattern: string, pathname: string) {
  const patternSegments = normalizePath(pattern).split("/");
  const pathSegments = normalizePath(pathname).split("/");
  if (patternSegments.length !== pathSegments.length) {
    return false;
  }
  return patternSegments.every((segment, index) => segment.startsWith(":") ? pathSegments[index] !== "" : segment === pathSegments[index]);
}

function normalizePath(pathname: string) {
  const normalized = pathname.replace(/\/+$/, "");
  return normalized || "/";
}
