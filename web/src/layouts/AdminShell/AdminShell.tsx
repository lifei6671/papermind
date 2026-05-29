import { Button } from "../../components/ui/Button";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { ArrowLeftRight, Home, LogOut, Settings } from "lucide-react";
import { type AuthSession, useSession } from "../../auth/session-context";
import { routeVisibleForRole, type AdminRoute, type AdminRouteGroup } from "../../app/routes";
import "./AdminShell.css";

type AdminShellProps = {
  routes: AdminRoute[];
};

const groupLabels: Record<Exclude<AdminRouteGroup, "hidden">, string> = {
  platform: "平台运营",
  tenant: "租户空间",
  exam: "考试业务",
};

export function AdminShell({ routes }: AdminShellProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const { session, signOut } = useSession();
  const visibleGroups = Object.keys(groupLabels) as Array<Exclude<AdminRouteGroup, "hidden">>;
  const menuRoutes = routes.filter((route) =>
    routeVisibleForRole(route, session?.user.role, session?.profileSpaces ?? []),
  );
  const sidebarIdentity = buildSidebarIdentity(session);

  const renderRouteLink = (route: AdminRoute) => {
    const Icon = route.icon;

    return (
      <NavLink
        className={({ isActive }) => (isActive ? "menu-link menu-link--active" : "menu-link")}
        end={route.path === "/"}
        key={route.path}
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
      <aside className="sidebar" aria-label="后台导航">
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
              to="/settings/profile"
            >
              <Settings aria-hidden="true" size={15} strokeWidth={1.8} />
            </NavLink>
          </div>

          <Button className="sidebar__home" onClick={() => navigate(sidebarIdentity.actionPath)} type="button">
            <sidebarIdentity.ActionIcon aria-hidden="true" size={15} />
            {sidebarIdentity.actionLabel}
          </Button>
          <Button
            className="sidebar__logout"
            onClick={() => {
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

function routeLinkTarget(route: AdminRoute, currentSearch: string) {
  const tenantID = new URLSearchParams(currentSearch).get("tenant_id");
  if ((route.group === "tenant" || route.group === "exam") && tenantID) {
    return `${route.path}?tenant_id=${encodeURIComponent(tenantID)}`;
  }
  return route.path;
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
