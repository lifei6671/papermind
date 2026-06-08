import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { adminRoutes, buildAdminRoutes } from "../../app/routes";
import { School } from "lucide-react";
import type { AdminRoute } from "../../app/routes";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import { SessionProvider } from "../../auth/session";
import { AdminShell } from "./AdminShell";

afterEach(() => {
  window.localStorage.clear();
});

test("退出登录时清理已保存登录态", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      user: { displayName: "平台管理员", role: "platform_admin", userID: 1 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
            <Route path="/login" element={<div>登录页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "退出登录" }));

  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toBeNull();
});

test("租户业务导航保留当前目标租户 ID", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/spaces?tenant_id=10"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/spaces" element={<div>空间页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: /用户管理/ })).toHaveAttribute("href", "/users?tenant_id=10");
});

test("租户管理员导航不保留 URL 中陈旧的空间 ID", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/papers?space_id=301"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/papers" element={<div>试卷页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: /概览/ })).toHaveAttribute("href", "/");
  expect(screen.getByRole("link", { name: /试卷/ })).toHaveAttribute("href", "/papers");
});

test("旧试卷预览 fallback 页左侧菜单归属试卷", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "青藤一中",
        spaceID: 301,
        role: "teacher",
        status: "enabled",
      }],
      user: { displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/papers/100/preview?space_id=301"]}>
        <Routes>
          <Route element={<AdminShell routes={buildAdminRoutes({
            displayName: "阅卷教师",
            role: "teacher",
            tenantID: 10,
            userID: 3,
          }, 301, [{
            id: 1,
            tenantID: 10,
            tenantName: "青藤一中",
            spaceID: 301,
            role: "teacher",
            status: "enabled",
          }])} />}>
            <Route path="/papers/:paperID/preview" element={<div>试卷预览页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: "试卷" })).toHaveClass("menu-link--active");
  expect(screen.getByRole("link", { name: "试卷" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("link", { name: "考试列表" })).not.toHaveClass("menu-link--active");
  expect(screen.getByRole("link", { name: "考试列表" })).not.toHaveAttribute("aria-current");
  expect(screen.getByRole("main")).toHaveClass("workspace__content--paper-preview");
});

test("考试详情页使用试卷预览宽度容器", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "青藤一中",
        spaceID: 301,
        role: "teacher",
        status: "enabled",
      }],
      user: { displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route element={<AdminShell routes={buildAdminRoutes({
            displayName: "阅卷教师",
            role: "teacher",
            tenantID: 10,
            userID: 3,
          }, 301, [{
            id: 1,
            tenantID: 10,
            tenantName: "青藤一中",
            spaceID: 301,
            role: "teacher",
            status: "enabled",
          }])} />}>
            <Route path="/exams/:examID" element={<div>考试详情页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("main")).toHaveClass("workspace__content--paper-preview");
});

test("考试详情兼容预览路由同样使用试卷预览宽度容器", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "青藤一中",
        spaceID: 301,
        role: "teacher",
        status: "enabled",
      }],
      user: { displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/exams/8/preview?space_id=301"]}>
        <Routes>
          <Route element={<AdminShell routes={buildAdminRoutes({
            displayName: "阅卷教师",
            role: "teacher",
            tenantID: 10,
            userID: 3,
          }, 301, [{
            id: 1,
            tenantID: 10,
            tenantName: "青藤一中",
            spaceID: 301,
            role: "teacher",
            status: "enabled",
          }])} />}>
            <Route path="/exams/:examID/preview" element={<div>考试详情兼容预览页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("main")).toHaveClass("workspace__content--paper-preview");
  expect(screen.getByRole("link", { name: "考试列表" })).toHaveClass("menu-link--active");
});

test("阅卷和成绩导航保留当前考试 ID", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        spaceID: 301,
        role: "teacher",
        status: "enabled",
      }],
      user: { displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/results?tenant_id=10&space_id=301&exam_id=42"]}>
        <Routes>
          <Route element={<AdminShell routes={buildAdminRoutes({
            displayName: "阅卷教师",
            role: "teacher",
            tenantID: 10,
            userID: 3,
          }, 301, [{
            id: 1,
            tenantID: 10,
            spaceID: 301,
            role: "teacher",
            status: "enabled",
          }], 42)} />}>
            <Route path="/results" element={<div>成绩页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: /概览/ })).toHaveAttribute("href", "/?tenant_id=10&space_id=301");
  expect(screen.getByRole("link", { name: /阅卷中心/ })).toHaveAttribute(
    "href",
    "/grading?tenant_id=10&space_id=301&exam_id=42",
  );
  expect(screen.getByRole("link", { name: /成绩/ })).toHaveAttribute(
    "href",
    "/results?tenant_id=10&space_id=301&exam_id=42",
  );
});

test("考试详情页阅卷和成绩导航从路径保留当前考试 ID", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "青藤一中",
        spaceID: 301,
        role: "teacher",
        status: "enabled",
      }],
      user: { displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route element={<AdminShell routes={buildAdminRoutes({
            displayName: "阅卷教师",
            role: "teacher",
            tenantID: 10,
            userID: 3,
          }, 301, [{
            id: 1,
            tenantID: 10,
            tenantName: "青藤一中",
            spaceID: 301,
            role: "teacher",
            status: "enabled",
          }], 8)} />}>
            <Route path="/exams/:examID" element={<div>考试详情页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: /阅卷中心/ })).toHaveAttribute(
    "href",
    "/grading?space_id=301&exam_id=8",
  );
  expect(screen.getByRole("link", { name: /成绩/ })).toHaveAttribute(
    "href",
    "/results?space_id=301&exam_id=8",
  );
});

test("平台管理员不展示租户空间菜单", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      user: { displayName: "平台管理员", role: "platform_admin", userID: 1 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.queryByText("租户空间")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /空间管理/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /用户管理/ })).not.toBeInTheDocument();
  expect(screen.queryByText("考试业务")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /阅卷中心/ })).not.toBeInTheDocument();
});

test("租户管理员展示租户空间菜单", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByText("租户空间")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /概览/ })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /空间管理/ })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /用户管理/ })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /空间成员/ })).not.toBeInTheDocument();
  expect(screen.getByText("租户管理员")).toBeInTheDocument();
  expect(screen.getByText("明德学校 租户后台")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "切换租户" })).toBeInTheDocument();
});

test("小屏幕菜单按钮以抽屉方式展开后台导航", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  const menuButton = screen.getByRole("button", { name: "展开后台导航" });
  expect(menuButton).toHaveAttribute("aria-expanded", "false");

  await user.click(menuButton);

  expect(menuButton).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByLabelText("后台导航")).toHaveClass("sidebar--mobile-open");
  expect(screen.getByRole("button", { name: "关闭后台导航" })).toHaveAttribute(
    "data-placement",
    "drawer-right",
  );
});

test("移动抽屉打开时不继承桌面图标栏状态", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/papers"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/papers" element={<div>试卷页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  const sidebar = screen.getByLabelText("后台导航");
  await user.click(within(sidebar).getByRole("button", { name: "收起后台导航" }));

  expect(sidebar).toHaveClass("sidebar--collapsed");
  expect(sidebar).toHaveClass("sidebar--icon-only");

  await user.click(screen.getByRole("button", { name: "展开后台导航", expanded: false }));

  expect(sidebar).toHaveClass("sidebar--mobile-open");
  expect(sidebar).not.toHaveClass("sidebar--collapsed");
  expect(sidebar).not.toHaveClass("sidebar--icon-only");
  expect(sidebar.querySelector(".brand-rail")).not.toHaveClass("brand-rail--icon-only");
  expect(sidebar.querySelector(".menu-link--icon-only")).toBeNull();
  expect(sidebar.querySelector(".sidebar__home")).not.toHaveClass("sidebar__home--icon-only");
  expect(sidebar.querySelector(".sidebar__logout")).not.toHaveClass("sidebar__logout--icon-only");
});

test("桌面侧边栏支持收起为图标栏并可再次展开", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  const { container } = render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/papers"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/papers" element={<div>试卷页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  const shell = container.querySelector(".admin-shell");
  const sidebar = screen.getByLabelText("后台导航");
  const collapseButton = within(sidebar).getByRole("button", { name: "收起后台导航" });
  expect(shell).not.toHaveClass("admin-shell--sidebar-collapsed");
  expect(sidebar).not.toHaveClass("sidebar--collapsed");

  await user.click(collapseButton);

  expect(shell).toHaveClass("admin-shell--sidebar-collapsed");
  expect(sidebar).toHaveClass("sidebar--collapsed");
  expect(within(sidebar).getByRole("button", { name: "展开后台导航" })).toBeInTheDocument();

  await user.click(within(sidebar).getByRole("button", { name: "展开后台导航" }));

  expect(shell).not.toHaveClass("admin-shell--sidebar-collapsed");
  expect(sidebar).not.toHaveClass("sidebar--collapsed");
});

test("桌面侧边栏折叠状态会保存到本地并在重新加载后恢复", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  const renderShell = () => render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/papers"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/papers" element={<div>试卷页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  const firstRender = renderShell();
  await user.click(within(screen.getByLabelText("后台导航")).getByRole("button", { name: "收起后台导航" }));
  firstRender.unmount();

  const { container } = renderShell();
  const shell = container.querySelector(".admin-shell");
  const sidebar = screen.getByLabelText("后台导航");

  expect(shell).toHaveClass("admin-shell--sidebar-collapsed");
  expect(sidebar).toHaveClass("sidebar--collapsed");
  expect(sidebar).toHaveClass("sidebar--icon-only");
});

test("桌面侧边栏收起后需要重新移入才临时展开", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/papers"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/papers" element={<div>试卷页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  const sidebar = screen.getByLabelText("后台导航");
  await user.hover(sidebar);
  await user.click(within(sidebar).getByRole("button", { name: "收起后台导航" }));

  expect(sidebar).toHaveClass("sidebar--collapsed");
  expect(sidebar).toHaveClass("sidebar--icon-only");
  expect(sidebar.querySelector(".brand-rail")).toHaveClass("brand-rail--icon-only");
  expect(sidebar.querySelector(".menu-link--icon-only")).not.toBeNull();
  expect(sidebar.querySelector(".sidebar__home")).toHaveClass("sidebar__home--icon-only");
  expect(sidebar.querySelector(".sidebar__logout")).toHaveClass("sidebar__logout--icon-only");
  expect(sidebar).not.toHaveClass("sidebar--hover-open");

  await user.unhover(sidebar);
  await user.hover(sidebar);

  expect(sidebar).not.toHaveClass("sidebar--icon-only");
  expect(sidebar.querySelector(".brand-rail")).not.toHaveClass("brand-rail--icon-only");
  expect(sidebar.querySelector(".menu-link--icon-only")).toBeNull();
  expect(sidebar.querySelector(".sidebar__home")).not.toHaveClass("sidebar__home--icon-only");
  expect(sidebar.querySelector(".sidebar__logout")).not.toHaveClass("sidebar__logout--icon-only");
  expect(sidebar).toHaveClass("sidebar--hover-open");
});

test("租户用户点击切换租户进入租户空间选择页", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 0,
        role: "tenant_admin",
        status: "enabled",
      }],
      user: { displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
            <Route path="/tenant-entry" element={<div>租户空间选择页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "切换租户" }));

  expect(screen.getByText("租户空间选择页")).toBeInTheDocument();
});

test("教师展示考试业务菜单", () => {
	window.localStorage.setItem(
		SESSION_STORAGE_KEY,
    JSON.stringify({
      user: { displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByText("考试业务")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /概览/ })).toBeInTheDocument();
  expect(screen.queryByText("平台运营")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /租户管理/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /平台配置/ })).not.toBeInTheDocument();
	expect(screen.getByRole("link", { name: /阅卷中心/ })).toBeInTheDocument();
	expect(screen.getByRole("link", { name: /成绩/ })).toBeInTheDocument();
});

test("菜单可由授权空间驱动而不是把 space_admin 写入 session role", () => {
	const routes: AdminRoute[] = [{
		description: "管理当前空间成员",
		element: <div>空间成员页</div>,
		group: "tenant",
		icon: School,
		label: "空间成员",
		menuRoles: [],
		path: "/space-members",
		spaceMemberRoles: ["space_admin"],
	}];
	window.localStorage.setItem(
		SESSION_STORAGE_KEY,
		JSON.stringify({
			profileSpaces: [{
				id: 1,
				tenantID: 10,
				spaceID: 301,
				role: "space_admin",
				status: "enabled",
			}],
			user: { displayName: "空间管理员", role: "teacher", tenantID: 10, userID: 3 },
		}),
	);

	render(
		<SessionProvider>
			<MemoryRouter initialEntries={["/"]}>
				<Routes>
					<Route element={<AdminShell routes={routes} />}>
						<Route path="/" element={<div>概览页面</div>} />
					</Route>
				</Routes>
			</MemoryRouter>
		</SessionProvider>,
	);

	expect(screen.getByRole("link", { name: /空间成员/ })).toBeInTheDocument();
});

test("真实空间成员菜单可由授权空间列表驱动", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        spaceID: 301,
        role: "space_admin",
        status: "enabled",
      }],
      user: { displayName: "空间管理员", role: "teacher", tenantID: 10, userID: 3 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={buildAdminRoutes({
            displayName: "空间管理员",
            role: "teacher",
            tenantID: 10,
            userID: 3,
          })} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: /空间成员/ })).toHaveAttribute("href", "/space-members");
  expect(screen.queryByRole("link", { name: /空间管理/ })).not.toBeInTheDocument();
});
