import type { LucideIcon } from "lucide-react";
import {
  BarChart3,
  BookOpenCheck,
  Building2,
  ClipboardList,
  FileStack,
  GraduationCap,
  LibraryBig,
  PenLine,
  School,
  Settings,
  Trophy,
  Upload,
  Users,
} from "lucide-react";
import { DashboardPage } from "../pages/Dashboard/DashboardPage";
import { ExamEntryPage } from "../pages/Exam/ExamEntryPage";
import { ExamManagementPage } from "../pages/Exam/ExamManagementPage";
import { PaperAssemblyPage } from "../pages/Exam/PaperAssemblyPage";
import { QuestionBankPage } from "../pages/Exam/QuestionBankPage";
import { QuestionImportPage } from "../pages/Exam/QuestionImportPage";
import { GradingPage } from "../pages/Grading/GradingPage";
import { PlatformSettingsPage } from "../pages/Platform/PlatformSettingsPage";
import { TenantManagementPage } from "../pages/Platform/TenantManagementPage";
import { ProfileSettingsPage } from "../pages/Profile/ProfileSettingsPage";
import { ResultsPage } from "../pages/Results/ResultsPage";
import { SpaceManagementPage } from "../pages/Tenant/SpaceManagementPage";
import { UserManagementPage } from "../pages/Tenant/UserManagementPage";
import type { ActorRole } from "../api/grading";
import type { ProfileSpaceAuthorization, SessionUser } from "../auth/session-context";
import { TenantScopedRoute } from "./TenantScopedRoute";

export type AdminRouteGroup = "platform" | "tenant" | "exam" | "hidden";

export type AdminRoute = {
  path: string;
  label: string;
  description: string;
  icon: LucideIcon;
  element: React.ReactElement;
  group: AdminRouteGroup;
  badge?: string;
  menuRoles?: string[];
  // 需要空间内身份的菜单项通过 profileSpaces 授权，不把 space_admin 混入租户级角色。
  spaceMemberRoles?: ProfileSpaceAuthorization["role"][];
};

export const tenantAdminRoles = ["tenant_admin"];
export const examBusinessRoles = ["tenant_admin", "teacher"];

export function buildAdminRoutes(user?: SessionUser | null): AdminRoute[] {
  const sessionTenantID = user?.tenantID;
  const actorID = user?.userID ?? 0;
  const actorRole = routeActorRole(user?.role);

  return [
  {
    path: "/",
    label: "概览",
    description: "租户、考试、阅卷和成绩风险总览",
    icon: BarChart3,
    element: <DashboardPage />,
    group: "platform",
  },
  {
    path: "/tenants",
    label: "租户管理",
    description: "创建租户、维护租户码、配置注册开关",
    icon: Building2,
    element: <TenantManagementPage />,
    group: "platform",
  },
  {
    path: "/platform-settings",
    label: "平台配置",
    description: "维护注册开关、平台公告和首版安全配置",
    icon: Settings,
    element: <PlatformSettingsPage />,
    group: "platform",
  },
  {
    path: "/spaces",
    label: "空间管理",
    description: "管理空间、成员、空间管理员和空间配置",
    icon: School,
    element: (
      <TenantScopedRoute
        render={(tenantID) => <SpaceManagementPage tenantID={tenantID} />}
        title="空间管理"
        user={user}
      />
    ),
    group: "tenant",
    menuRoles: tenantAdminRoles,
  },
  {
    path: "/users",
    label: "用户管理",
    description: "维护租户用户、导入用户和禁用提示",
    icon: Users,
    element: (
      <TenantScopedRoute
        render={(tenantID) => <UserManagementPage actorID={actorID} tenantID={tenantID} />}
        title="用户管理"
        user={user}
      />
    ),
    group: "tenant",
    menuRoles: tenantAdminRoles,
  },
  {
    path: "/questions",
    label: "题库",
    description: "维护题目、选项、解析、标签和导入任务",
    icon: LibraryBig,
    element: <QuestionBankPage tenantID={sessionTenantID ?? 0} />,
    group: "exam",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/imports",
    label: "题目导入",
    description: "上传 CSV/Excel 模板并查看导入错误行",
    icon: Upload,
    element: <QuestionImportPage tenantID={sessionTenantID ?? 0} />,
    group: "exam",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/papers",
    label: "试卷",
    description: "维护大题、手动组卷和规则组卷",
    icon: FileStack,
    element: <PaperAssemblyPage tenantID={sessionTenantID ?? 0} />,
    group: "exam",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/exams",
    label: "考试",
    description: "发布考试、配置范围、邀请码和结果策略",
    icon: ClipboardList,
    element: <ExamManagementPage tenantID={sessionTenantID ?? 0} />,
    group: "exam",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/grading",
    label: "阅卷中心",
    description: "处理简答题待阅卷、评语和成绩重算",
    icon: PenLine,
    element: <GradingPage actorID={actorID} actorRole={actorRole} tenantID={sessionTenantID ?? 0} />,
    group: "exam",
    badge: "P8 已就绪",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/results",
    label: "成绩",
    description: "查看成绩、配置发布、导出 CSV",
    icon: Trophy,
    element: <ResultsPage actorID={actorID} actorRole={actorRole} tenantID={sessionTenantID ?? 0} />,
    group: "exam",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/exam-entry",
    label: "考试端入口",
    description: "考生入口、邀请码、说明页和答题流程",
    icon: GraduationCap,
    element: <ExamEntryPage />,
    group: "exam",
    menuRoles: examBusinessRoles,
  },
  {
    path: "/settings/profile",
    label: "个人设置",
    description: "维护当前账号资料和默认空间",
    icon: BookOpenCheck,
    element: <ProfileSettingsPage />,
    group: "hidden",
  },
  ];
}

export function routeVisibleForRole(
  route: AdminRoute,
  role?: string,
  profileSpaces: ProfileSpaceAuthorization[] = [],
) {
  if (!route.menuRoles) {
    return true;
  }
  if (role && route.menuRoles.includes(role)) {
    return true;
  }

  // 空间管理员入口必须从启用的空间成员授权推导，不能把 space_admin 写成 session 角色。
  return !!route.spaceMemberRoles?.some((memberRole) =>
    profileSpaces.some((space) => space.status === "enabled" && space.role === memberRole),
  );
}

function routeActorRole(role?: string): ActorRole {
  switch (role) {
    case "teacher":
    case "student":
      return role;
    default:
      return "tenant_admin";
  }
}

export const adminRoutes: AdminRoute[] = buildAdminRoutes();
