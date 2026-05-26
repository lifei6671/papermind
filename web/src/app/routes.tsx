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
import { PlaceholderPage } from "../pages/Placeholder/PlaceholderPage";

export type AdminRouteGroup = "platform" | "tenant" | "exam" | "hidden";

export type AdminRoute = {
  path: string;
  label: string;
  description: string;
  icon: LucideIcon;
  element: React.ReactElement;
  group: AdminRouteGroup;
  badge?: string;
};

export const adminRoutes: AdminRoute[] = [
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
    element: <PlaceholderPage title="租户管理" description="租户列表、创建租户、Logo、描述和租户码将在这里收口。" />,
    group: "platform",
  },
  {
    path: "/platform-settings",
    label: "平台配置",
    description: "维护注册开关、平台公告和首版安全配置",
    icon: Settings,
    element: <PlaceholderPage title="平台配置" description="平台配置后续会接入 P3 已完成的配置服务。" />,
    group: "platform",
  },
  {
    path: "/spaces",
    label: "空间管理",
    description: "管理空间、成员、空间管理员和空间配置",
    icon: School,
    element: <PlaceholderPage title="空间管理" description="空间列表、成员角色和空间管理员不变式提示将在这里实现。" />,
    group: "tenant",
  },
  {
    path: "/users",
    label: "用户管理",
    description: "维护租户用户、导入用户和禁用提示",
    icon: Users,
    element: <PlaceholderPage title="用户管理" description="用户创建、导入、头像和禁用影响范围后续接入真实接口。" />,
    group: "tenant",
  },
  {
    path: "/questions",
    label: "题库",
    description: "维护题目、选项、解析、标签和导入任务",
    icon: LibraryBig,
    element: <PlaceholderPage title="题库" description="题目列表、在线出题、标签管理和题目导入将在这里推进。" />,
    group: "exam",
  },
  {
    path: "/imports",
    label: "题目导入",
    description: "上传 CSV/Excel 模板并查看导入错误行",
    icon: Upload,
    element: <PlaceholderPage title="题目导入" description="文件上传组件和导入结果反馈属于 P9.1/P9.4 后续任务。" />,
    group: "exam",
  },
  {
    path: "/papers",
    label: "试卷",
    description: "维护大题、手动组卷和规则组卷",
    icon: FileStack,
    element: <PlaceholderPage title="试卷" description="手动组卷、rule_fixed、rule_live 和预检查后续在这里展开。" />,
    group: "exam",
  },
  {
    path: "/exams",
    label: "考试",
    description: "发布考试、配置范围、邀请码和结果策略",
    icon: ClipboardList,
    element: <PlaceholderPage title="考试" description="考试草稿、发布范围、邀请码和时间窗口将在这里形成闭环。" />,
    group: "exam",
  },
  {
    path: "/grading",
    label: "阅卷中心",
    description: "处理简答题待阅卷、评语和成绩重算",
    icon: PenLine,
    element: <PlaceholderPage title="阅卷中心" description="待阅卷列表和简答题评分会接入 P8 阅卷服务。" />,
    group: "exam",
    badge: "P8 已就绪",
  },
  {
    path: "/results",
    label: "成绩",
    description: "查看成绩、配置发布、导出 CSV",
    icon: Trophy,
    element: <PlaceholderPage title="成绩" description="成绩发布可见性、latest/highest 和导出文件会在这里呈现。" />,
    group: "exam",
  },
  {
    path: "/exam-entry",
    label: "考试端入口",
    description: "考生入口、邀请码、说明页和答题流程",
    icon: GraduationCap,
    element: <PlaceholderPage title="考试端入口" description="考试端会独立承载倒计时、自动保存和提交确认。" />,
    group: "exam",
  },
  {
    path: "/settings/profile",
    label: "个人设置",
    description: "维护当前账号资料和默认空间",
    icon: BookOpenCheck,
    element: <PlaceholderPage title="个人设置" description="当前用户资料和默认空间后续接入登录态。" />,
    group: "hidden",
  },
];
