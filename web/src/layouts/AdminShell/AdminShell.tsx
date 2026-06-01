import { Button } from "../../components/ui/Button";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useState } from "react";
import { ArrowLeftRight, Home, LogOut, Menu, Settings, X } from "lucide-react";
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

export function AdminShell({ routes }: AdminShellProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const { session, signOut } = useSession();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const visibleGroups = Object.keys(groupLabels) as Array<Exclude<AdminRouteGroup, "hidden">>;
  const scopedSpaceID = session?.selectedSpaceID ?? positiveID(new URLSearchParams(location.search).get("space_id"));
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
        className={({ isActive }) => (isActive ? "menu-link menu-link--active" : "menu-link")}
        end={route.path === "/"}
        key={route.path}
        onClick={() => setSidebarOpen(false)}
        to={routeLinkTarget(route, location.search)}
      >
        <Icon aria-hidden="true" size={18} strokeWidth={1.8} />
        <span>{route.label}</span>
        {route.badge && <em>{route.badge}</em>}
      </NavLink>
    );
  };

  return (
    <div className="admin-shell">
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
        className={sidebarOpen ? "sidebar sidebar--mobile-open" : "sidebar"}
        id="admin-sidebar"
        aria-label="后台导航"
      >
        <div className="brand-rail">
          <div className="brand-mark" aria-hidden="true">
            P
          </div>
          <span className="brand-word">Papermind</span>
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
            className="sidebar__home"
            onClick={() => {
              setSidebarOpen(false);
              navigate(sidebarIdentity.actionPath);
            }}
            type="button"
          >
            <sidebarIdentity.ActionIcon aria-hidden="true" size={15} />
            {sidebarIdentity.actionLabel}
          </Button>
          <Button
            className="sidebar__logout"
            onClick={() => {
              setSidebarOpen(false);
              signOut();
              navigate("/login", { replace: true });
            }}
            type="button"
          >
            <LogOut aria-hidden="true" size={15} />
            退出登录
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

function positiveID(value: string | null) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

function routeLinkTarget(route: AdminRoute, currentSearch: string) {
  const currentParams = new URLSearchParams(currentSearch);
  const nextParams = new URLSearchParams();
  const tenantID = currentParams.get("tenant_id");
  const spaceID = currentParams.get("space_id");
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
