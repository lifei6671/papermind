import { Button } from "../../components/ui/Button";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useState } from "react";
import { ArrowLeftRight, ChevronLeft, ChevronRight, Home, LogOut, Menu, Settings, X } from "lucide-react";
import { type AuthSession, useSession } from "../../auth/session-context";
import { routeVisibleForRole, type AdminRoute, type AdminRouteGroup } from "../../app/routes";
import "./AdminShell.css";

type AdminShellProps = {
  routes: AdminRoute[];
};

const groupLabels: Record<Exclude<AdminRouteGroup, "hidden">, string> = {
  overview: "总览",
  platform: "平台运营",
  tenant: "租户空间",
  exam: "考试业务",
};
const SIDEBAR_COLLAPSED_STORAGE_KEY = "papermind:admin-sidebar-collapsed";

export function AdminShell({ routes }: AdminShellProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const { session, signOut } = useSession();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(readSidebarCollapsedPreference);
  const [sidebarHovered, setSidebarHovered] = useState(false);
  const visibleGroups = Object.keys(groupLabels) as Array<Exclude<AdminRouteGroup, "hidden">>;
  const scopedSpaceID = scopedSpaceIDFromSession(session, new URLSearchParams(location.search));
  const iconOnly = sidebarCollapsed && !sidebarHovered && !sidebarOpen;
  const menuRoutes = routes.filter((route) =>
    routeVisibleForRole(route, session?.user.role, session?.profileSpaces ?? [], {
      tenantID: session?.user.tenantID,
      spaceID: scopedSpaceID,
    }),
  );
  const sidebarIdentity = buildSidebarIdentity(session);

  const renderRouteLink = (route: AdminRoute) => {
    const Icon = route.icon;

    return (
      <NavLink
        className={({ isActive }) => {
          const classes = ["menu-link"];
          if (isActive) {
            classes.push("menu-link--active");
          }
          if (iconOnly) {
            classes.push("menu-link--icon-only");
          }
          return classes.join(" ");
        }}
        end={route.path === "/"}
        key={route.path}
        onClick={() => setSidebarOpen(false)}
        to={routeLinkTarget(route, location.search, session)}
      >
        <Icon aria-hidden="true" size={18} strokeWidth={1.8} />
        <span>{route.label}</span>
        {route.badge && <em>{route.badge}</em>}
      </NavLink>
    );
  };

  return (
    <div className={sidebarCollapsed ? "admin-shell admin-shell--sidebar-collapsed" : "admin-shell"}>
      <button
        aria-controls="admin-sidebar"
        aria-expanded={sidebarOpen}
        aria-label={sidebarOpen ? "关闭后台导航" : "展开后台导航"}
        className="mobile-nav-toggle"
        data-placement={sidebarOpen ? "drawer-right" : "page-left"}
        onClick={() => setSidebarOpen((open) => !open)}
        type="button"
      >
        {sidebarOpen ? <X aria-hidden="true" size={20} /> : <Menu aria-hidden="true" size={20} />}
      </button>

      {sidebarOpen && (
        <button
          aria-label="关闭导航遮罩"
          className="sidebar-backdrop"
          onClick={() => setSidebarOpen(false)}
          type="button"
        />
      )}

      <aside
        className={buildSidebarClassName(sidebarOpen, sidebarCollapsed, sidebarHovered)}
        id="admin-sidebar"
        aria-label="后台导航"
        onMouseEnter={() => {
          if (sidebarCollapsed) {
            setSidebarHovered(true);
          }
        }}
        onMouseLeave={() => setSidebarHovered(false)}
      >
        <div className={iconOnly ? "brand-rail brand-rail--icon-only" : "brand-rail"}>
          <div className="brand-mark" aria-hidden="true">
            P
          </div>
          <span className="brand-word">Papermind</span>
          <button
            aria-label={sidebarCollapsed ? "展开后台导航" : "收起后台导航"}
            className="sidebar-desktop-toggle"
            onClick={() => {
              setSidebarCollapsed((current) => {
                const next = !current;
                writeSidebarCollapsedPreference(next);
                return next;
              });
              setSidebarHovered(false);
            }}
            type="button"
          >
            {sidebarCollapsed ? <ChevronRight aria-hidden="true" size={16} /> : <ChevronLeft aria-hidden="true" size={16} />}
          </button>
        </div>

        <div className="menu-panel">
          {visibleGroups.map((group) => {
            const groupRoutes = menuRoutes.filter((route) => route.group === group);
            if (!groupRoutes.length) {
              return null;
            }

            return (
              <div className="menu-panel__section" key={group}>
                <span className="eyebrow">{groupLabels[group]}</span>
                <nav className="menu-panel__nav" aria-label={groupLabels[group]}>
                  {groupRoutes.map(renderRouteLink)}
                </nav>
              </div>
            );
          })}

          <div className="menu-panel__note">
            <div>
              <span>{sidebarIdentity.title}</span>
              <p>{sidebarIdentity.description}</p>
            </div>
            <NavLink
              aria-label="个人设置"
              className={({ isActive }) =>
                isActive ? "menu-panel__settings menu-panel__settings--active" : "menu-panel__settings"
              }
              onClick={() => setSidebarOpen(false)}
              to="/settings/profile"
            >
              <Settings aria-hidden="true" size={15} strokeWidth={1.8} />
            </NavLink>
          </div>

          <Button
            className={iconOnly ? "sidebar__home sidebar__home--icon-only" : "sidebar__home"}
            onClick={() => {
              setSidebarOpen(false);
              navigate(sidebarIdentity.actionPath);
            }}
            type="button"
          >
            <sidebarIdentity.ActionIcon aria-hidden="true" size={15} />
            <span>{sidebarIdentity.actionLabel}</span>
          </Button>
          <Button
            className={iconOnly ? "sidebar__logout sidebar__logout--icon-only" : "sidebar__logout"}
            onClick={() => {
              setSidebarOpen(false);
              signOut();
              navigate("/login", { replace: true });
            }}
            type="button"
          >
            <LogOut aria-hidden="true" size={15} />
            <span>退出登录</span>
          </Button>
        </div>
      </aside>

      <div className="workspace">
        <main className="workspace__content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function buildSidebarClassName(sidebarOpen: boolean, sidebarCollapsed: boolean, sidebarHovered: boolean) {
  const classes = ["sidebar"];
  if (sidebarOpen) {
    classes.push("sidebar--mobile-open");
  }
  if (!sidebarOpen && sidebarCollapsed) {
    classes.push("sidebar--collapsed");
  }
  if (!sidebarOpen && sidebarCollapsed && !sidebarHovered) {
    classes.push("sidebar--icon-only");
  }
  if (!sidebarOpen && sidebarCollapsed && sidebarHovered) {
    classes.push("sidebar--hover-open");
  }
  return classes.join(" ");
}

function readSidebarCollapsedPreference() {
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

function writeSidebarCollapsedPreference(collapsed: boolean) {
  try {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, String(collapsed));
  } catch {
    // 侧边栏状态只是偏好设置，本地存储不可用时不阻断导航。
  }
}

function positiveID(value: string | null) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

function scopedSpaceIDFromSession(session: AuthSession | null, searchParams: URLSearchParams) {
  if (!session || session.user.role === "platform_admin" || session.user.role === "tenant_admin") {
    return undefined;
  }
  return session.selectedSpaceID ?? positiveID(searchParams.get("space_id"));
}

function routeLinkTarget(route: AdminRoute, currentSearch: string, session: AuthSession | null) {
  const currentParams = new URLSearchParams(currentSearch);
  const nextParams = new URLSearchParams();
  const tenantID = currentParams.get("tenant_id");
  const spaceID = shouldPreserveSpaceScope(session) ? currentParams.get("space_id") : null;
  const examID = currentParams.get("exam_id");
  if ((route.group === "overview" || route.group === "tenant" || route.group === "exam") && tenantID) {
    nextParams.set("tenant_id", tenantID);
  }
  if ((route.group === "overview" || route.group === "exam") && spaceID) {
    nextParams.set("space_id", spaceID);
  }
  if (routeNeedsExamContext(route.path) && examID) {
    nextParams.set("exam_id", examID);
  }
  const query = nextParams.toString();
  return query ? `${route.path}?${query}` : route.path;
}

function shouldPreserveSpaceScope(session: AuthSession | null) {
  return session?.user.role !== "platform_admin" && session?.user.role !== "tenant_admin";
}

function routeNeedsExamContext(path: string) {
  return path === "/grading" || path === "/results";
}

function buildSidebarIdentity(session: AuthSession | null) {
  if (!session || session.user.role === "platform_admin") {
    return {
      ActionIcon: Home,
      actionLabel: "回到概览",
      actionPath: "/",
      description: "本地管理端 UI 骨架预览",
      title: "平台管理员",
    };
  }

  const currentTenant = session.profileSpaces?.find((space) => space.tenantID === session.user.tenantID);

  return {
    ActionIcon: ArrowLeftRight,
    actionLabel: "切换租户",
    actionPath: "/tenant-entry",
    description: currentTenant?.tenantName ? `${currentTenant.tenantName} 租户后台` : "租户管理后台",
    title: roleLabel(session.user.role),
  };
}

function roleLabel(role: AuthSession["user"]["role"]) {
  switch (role) {
    case "tenant_admin":
      return "租户管理员";
    case "teacher":
      return "教师";
    case "student":
      return "学生";
    case "tenant_user":
      return "租户用户";
    case "platform_admin":
      return "平台管理员";
  }
}
