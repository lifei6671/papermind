import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, vi } from "vitest";
import { act } from "react";
import { MemoryRouter } from "react-router-dom";
import { App } from "./App";
import { AppProviders } from "./providers";

const originalMatchMedia = window.matchMedia;

afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
  Object.defineProperty(window, "matchMedia", {
    value: originalMatchMedia,
    writable: true,
  });
});

function mockExamViewport(matchesNarrow: boolean) {
  Object.defineProperty(window, "matchMedia", {
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query === "(max-width: 1100px)" ? matchesNarrow : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
    writable: true,
  });
}

function renderApp(initialEntries: string[]) {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={initialEntries}>
        <App />
      </MemoryRouter>
    </AppProviders>,
  );
}

function storePlatformSession() {
  window.localStorage.setItem("papermind.session.v1", JSON.stringify({
    accessToken: "session-token",
    refreshToken: "session-token",
    user: { userID: 1, displayName: "admin", role: "platform_admin" },
  }));
}

function storeTenantStudentSession() {
  window.localStorage.setItem("papermind.session.v1", JSON.stringify({
    accessToken: "tenant-session-token",
    refreshToken: "tenant-session-token",
    user: { userID: 20, displayName: "目标考生", role: "student", tenantID: 77 },
  }));
}

function storeTenantTeacherSession() {
  window.localStorage.setItem("papermind.session.v1", JSON.stringify({
    accessToken: "teacher-session-token",
    refreshToken: "teacher-session-token",
    user: { userID: 55, displayName: "阅卷教师", role: "teacher", tenantID: 77 },
  }));
}

function storeSpaceTeacherSession() {
  window.localStorage.setItem("papermind.session.v1", JSON.stringify({
    accessToken: "teacher-session-token",
    refreshToken: "teacher-session-token",
    profileSpaces: [{
      id: 1,
      tenantID: 77,
      spaceID: 301,
      role: "teacher",
      status: "enabled",
    }],
    user: { userID: 55, displayName: "阅卷教师", role: "teacher", tenantID: 77 },
  }));
}

function storeSpaceAdminTeacherSession() {
  window.localStorage.setItem("papermind.session.v1", JSON.stringify({
    accessToken: "teacher-session-token",
    refreshToken: "teacher-session-token",
    profileSpaces: [{
      id: 1,
      tenantID: 77,
      spaceID: 301,
      role: "space_admin",
      status: "enabled",
    }],
    user: { userID: 55, displayName: "空间管理员", role: "teacher", tenantID: 77 },
  }));
}

function storeTenantAdminSession() {
  window.localStorage.setItem("papermind.session.v1", JSON.stringify({
    accessToken: "tenant-admin-session-token",
    refreshToken: "tenant-admin-session-token",
    user: { userID: 88, displayName: "租户管理员", role: "tenant_admin", tenantID: 77 },
  }));
}

function mockStudentExamFetch() {
  return vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url === "/api/v1/exam-entry/exams/1/attempts/start" && init?.method === "POST") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: mockStartAttemptPayload(),
      }));
    }
    if (url.startsWith("/api/v1/exam-entry/attempts/99/answers/") && init?.method === "POST") {
      return new Response(JSON.stringify({ code: 0, message: "ok", data: { saved: true } }));
    }
    if (url === "/api/v1/exam-entry/attempts/99/submit" && init?.method === "POST") {
      return new Response(JSON.stringify({ code: 0, message: "ok", data: { submitted: true } }));
    }
    if (url === "/api/v1/exam-entry/results/99" && init?.method === "GET") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          attempt_id: 99,
          exam_id: 1,
          attempt_no: 1,
          objective_score: "56",
          subjective_score: "30",
          total_score: "86",
          analysis_visible: true,
        },
      }));
    }
    if (url === "/api/v1/exam-entry/attempts/99/events" && init?.method === "POST") {
      return new Response(JSON.stringify({ code: 0, message: "ok", data: { recorded: true } }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });
}

function mockStartAttemptPayload() {
  return {
    attempt: {
      id: 99,
      answer_deadline: 1779795600000,
    },
    exam_token: "exam-token",
    questions: [
      createAttemptQuestion(9001, 1, "一、单项选择题", "共20题，每题2分", "single", "服务端题干 1", "2", [
        { id: 101, key: "A", content: "豪放飘逸" },
        { id: 102, key: "B", content: "沉郁顿挫" },
      ]),
      createAttemptQuestion(9021, 21, "二、多项选择题", "共10题，每题2分", "multiple", "服务端题干 21", "2", [
        { id: 201, key: "A", content: "豪放飘逸" },
        { id: 202, key: "C", content: "想象奇特" },
      ]),
      createAttemptQuestion(9031, 31, "三、判断题", "共5题，每题1分", "judge", "服务端题干 31", "1", [
        { id: 301, key: "正确", content: "" },
        { id: 302, key: "错误", content: "" },
      ]),
      createAttemptQuestion(9036, 36, "三、填空题", "共5题，每题2分", "fill_blank", "服务端题干 36", "2", []),
      createAttemptQuestion(9041, 41, "四、简答题", "共5题，共30分", "short_text", "服务端题干 41", "10", []),
    ],
  };
}

function createAttemptQuestion(
  id: number,
  sortOrder: number,
  sectionName: string,
  sectionInstructions: string,
  type: string,
  title: string,
  score: string,
  options: Array<{ id: number; key: string; content: string }>,
) {
  return {
    id,
    sort_order: sortOrder,
    section: {
      name: sectionName,
      instructions: sectionInstructions,
    },
    question: {
      title,
      type,
    },
    options,
    score,
  };
}

test("渲染 Papermind 管理端基础骨架", () => {
  storePlatformSession();

  renderApp(["/"]);

  expect(screen.getByText("Papermind")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /租户管理/ })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /阅卷中心/ })).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "考试平台概览" })).toBeInTheDocument();
});

test("未登录访问后台路由会跳转登录页", () => {
  window.localStorage.removeItem("papermind.session.v1");

  renderApp(["/tenants"]);

  expect(screen.getByRole("heading", { name: "平台管理员登录" })).toBeInTheDocument();
});

test("后台接口返回未认证时自动退出并跳转登录页", async () => {
  storePlatformSession();
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify({ code: 40001, message: "请先登录平台管理员账号", data: null }), {
      headers: { "Content-Type": "application/json" },
      status: 401,
    }),
  );

  renderApp(["/tenants"]);

  expect(await screen.findByRole("heading", { name: "平台管理员登录" })).toBeInTheDocument();
  expect(window.localStorage.getItem("papermind.session.v1")).toBeNull();
});

test("平台管理员直接访问空间管理会回到概览且不请求租户接口", () => {
  storePlatformSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/spaces?tenant_id=10"]);

  expect(screen.getByRole("heading", { name: "考试平台概览" })).toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

test("平台管理员直接访问用户管理会回到概览且不请求租户接口", () => {
  storePlatformSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/users?tenant_id=10"]);

  expect(screen.getByRole("heading", { name: "考试平台概览" })).toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

test("平台管理员直接访问考试业务路由会回到概览", () => {
  storePlatformSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/grading?space_id=301&exam_id=1"]);

  expect(screen.getByRole("heading", { name: "考试平台概览" })).toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "阅卷中心" })).not.toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

test("租户用户直接访问平台治理路由会回到概览且不触发平台 API", () => {
  storeTenantTeacherSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/tenants"]);

  expect(screen.getByRole("heading", { name: "考试平台概览" })).toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "租户管理" })).not.toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

test("学生考试端与管理员后台路由隔离", async () => {
  mockStudentExamFetch();
  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(screen.getByRole("heading", { name: "在线考试" })).toBeInTheDocument();
  expect(screen.getByText("PaperMind")).toBeInTheDocument();
  expect(await screen.findByRole("button", { name: "考生" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "退出考试" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "答题卡" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "交 卷" })).toBeInTheDocument();
  expect(screen.queryByText("考生编号：S1001001")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "计算器" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "草稿纸" })).not.toBeInTheDocument();
  expect(screen.queryByLabelText("后台导航")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /租户管理/ })).not.toBeInTheDocument();
});

test("考生身份提示折叠在右侧用户菜单中", async () => {
  const user = userEvent.setup();
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  const profileButton = await screen.findByRole("button", { name: "考生" });
  expect(profileButton).toHaveAttribute("aria-expanded", "false");
  expect(screen.queryByRole("menu", { name: "考生信息" })).not.toBeInTheDocument();

  await user.click(profileButton);

  expect(profileButton).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByRole("menu", { name: "考生信息" })).toHaveTextContent("身份已校验");

  await user.click(screen.getByRole("button", { name: "收起题目列表" }));

  expect(profileButton).toHaveAttribute("aria-expanded", "false");
  expect(screen.queryByRole("menu", { name: "考生信息" })).not.toBeInTheDocument();
});

test("学生考试端窄屏辅助信息使用抽屉展开", async () => {
  const user = userEvent.setup();
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  const drawerToggle = await screen.findByRole("button", { name: "考试信息与答题卡" });
  expect(drawerToggle).toHaveAttribute("aria-expanded", "false");

  await user.click(drawerToggle);

  expect(drawerToggle).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByLabelText("考试辅助抽屉")).toHaveClass("exam-right-column--open");

  await user.click(screen.getByRole("button", { name: "关闭考试抽屉" }));

  expect(drawerToggle).toHaveAttribute("aria-expanded", "false");
});

test("H5 窄屏考试端直接使用真实 API 作答页", async () => {
  mockExamViewport(true);
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(screen.getByRole("heading", { name: "在线考试" })).toBeInTheDocument();
  expect(await screen.findByText("服务端题干 1")).toBeInTheDocument();
  expect(screen.queryByLabelText("开考前说明")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "考试信息与答题卡" })).toBeInTheDocument();
});

test("H5 窄屏答题卡通过考试抽屉展示并弹出确认交卷", async () => {
  const user = userEvent.setup();
  mockExamViewport(true);
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  await user.click(await screen.findByRole("button", { name: "考试信息与答题卡" }));

  expect(screen.getByLabelText("考试辅助抽屉")).toHaveClass("exam-right-column--open");
  expect(screen.getByRole("heading", { name: "答题卡" })).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "交 卷" }));

  expect(screen.getByRole("dialog", { name: "确认交卷" })).toBeInTheDocument();
  expect(screen.getByText("交卷后将无法继续作答，请确认是否交卷？")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "取消" }));

  expect(screen.queryByRole("dialog", { name: "确认交卷" })).not.toBeInTheDocument();
});

test("确认交卷通过交互弹窗展示", async () => {
  const user = userEvent.setup();
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(screen.queryByRole("dialog", { name: "确认交卷" })).not.toBeInTheDocument();

  await user.click(await screen.findByRole("button", { name: "交 卷" }));

  expect(screen.getByRole("dialog", { name: "确认交卷" })).toBeInTheDocument();
  expect(screen.getByText("交卷后将无法继续作答，请确认是否交卷？")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "取消" }));

  expect(screen.queryByRole("dialog", { name: "确认交卷" })).not.toBeInTheDocument();
});

test("自动保存提示不作为页面正文静态展示", () => {
  mockStudentExamFetch();
  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(screen.queryByRole("status", { name: "自动保存提示" })).not.toBeInTheDocument();
  expect(screen.queryByText("答案已自动保存")).not.toBeInTheDocument();
});

test("考试端支持全局题号切换和五类题型作答", async () => {
  const user = userEvent.setup();
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(await screen.findByText("服务端题干 1")).toBeInTheDocument();
  await user.click(within(screen.getByLabelText("题目列表")).getByRole("button", { name: "21" }));
  expect(screen.getByRole("heading", { name: "二、多项选择题" })).toBeInTheDocument();
  expect(screen.getByText((_content, node) => node?.textContent === "21 / 5")).toBeInTheDocument();
  await user.click(screen.getByLabelText("A.豪放飘逸"));
  await user.click(screen.getByLabelText("C.想象奇特"));

  await user.click(within(screen.getByLabelText("题目列表")).getByRole("button", { name: "31" }));
  expect(screen.getByRole("heading", { name: "三、判断题" })).toBeInTheDocument();
  await user.click(screen.getByLabelText(/正确/));

  await user.click(within(screen.getByLabelText("题目列表")).getByRole("button", { name: "36" }));
  expect(screen.getByRole("heading", { name: "三、填空题" })).toBeInTheDocument();
  await user.type(screen.getByLabelText("填空题答案"), "先天下之忧而忧");

  await user.click(within(screen.getByLabelText("题目列表")).getByRole("button", { name: "41" }));
  expect(screen.getByRole("heading", { name: "四、简答题" })).toBeInTheDocument();
  await user.type(screen.getByLabelText("简答题答案"), "体现了士大夫以天下为己任的担当。");

  expect(screen.getByRole("status", { name: "自动保存提示" })).toHaveTextContent("已自动保存");
});

test("考试端上报切屏事件并在交卷后展示成绩和解析", async () => {
  const user = userEvent.setup();
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(await screen.findByText("服务端题干 1")).toBeInTheDocument();
  act(() => {
    window.dispatchEvent(new Event("blur"));
  });

  expect(screen.getByRole("status", { name: "切屏事件上报" })).toHaveTextContent("已上报切屏事件");
  expect(screen.queryByText("题目解析已开放，可在成绩公布页查看解析内容。")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "交 卷" }));

  const dialog = screen.getByRole("dialog", { name: "确认交卷" });
  await user.click(within(dialog).getByRole("button", { name: "确认交卷" }));

  expect(screen.getByRole("heading", { name: "成绩可见页" })).toBeInTheDocument();
  expect(screen.getByText("总分 86 分")).toBeInTheDocument();
  expect(screen.getByText("题目解析已开放，可在成绩公布页查看解析内容。")).toBeInTheDocument();
});

test("简答题作答效果不作为当前考试页面正文展示", () => {
  mockStudentExamFetch();
  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(screen.queryByText("四、简答题（共30分）")).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText("请输入作答内容...")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "保存" })).not.toBeInTheDocument();
});

test("离开页面提醒通过确认弹窗展示", async () => {
  const user = userEvent.setup();
  mockStudentExamFetch();

  renderApp(["/student/exam?tenant_id=10&exam_id=1&user_id=20"]);

  expect(screen.queryByRole("dialog", { name: "离开页面提醒" })).not.toBeInTheDocument();

  await user.click(await screen.findByRole("button", { name: "退出考试" }));

  expect(screen.getByRole("dialog", { name: "离开页面提醒" })).toBeInTheDocument();
  expect(screen.getByText("检测到您将离开考试页面，请确认是否离开？")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "留在页面" }));

  expect(screen.queryByRole("dialog", { name: "离开页面提醒" })).not.toBeInTheDocument();
});

test("考试入口支持邀请码进入并跳转到考试端", async () => {
  const user = userEvent.setup();
  storeTenantStudentSession();
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url === "/api/v1/exam-entry/invite/resolve" && init?.method === "POST") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 42,
          tenant_id: 77,
          paper_id: 100,
          name: "高一语文期中考试",
          start_time: 1779792000000,
          end_time: 1779799200000,
          duration_minutes: 120,
          max_attempts: 1,
          result_strategy: "latest",
          publish_mode: "manual_publish",
          invite_code: "PM2026",
          status: "published",
        },
      }));
    }
    if (url === "/api/v1/exam-entry/exams/42/attempts/start" && init?.method === "POST") {
      expect(init.body).toBe(JSON.stringify({ tenant_id: 77 }));
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: mockStartAttemptPayload(),
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/exam-entry"]);

  expect(screen.getByRole("heading", { name: "考试入口" })).toBeInTheDocument();
  expect(screen.getByLabelText("邀请码")).toBeInTheDocument();
  expect(screen.getByText("考试说明")).toBeInTheDocument();
  await user.type(screen.getByLabelText("邀请码"), "PM2026");
  await user.click(screen.getByRole("button", { name: "进入考试" }));

  await waitFor(() => {
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "/api/v1/exam-entry/invite/resolve",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ invite_code: "PM2026" }),
      }),
    );
  });
  expect(await screen.findByRole("heading", { name: "在线考试" })).toBeInTheDocument();
  expect(await screen.findByRole("button", { name: "交 卷" })).toBeInTheDocument();
});

test("阅卷中心支持待阅卷列表、保存评分和完成阅卷", async () => {
  const user = userEvent.setup();
  storeSpaceTeacherSession();
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url.startsWith("/api/v1/grading/pending")) {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            attempt_id: 900,
            attempt_question_id: 901,
            student_name: "张三",
            space_name: "高一 1 班",
            exam_name: "高一语文期中考试",
            question_title: "岳阳楼记思想内涵",
            answer_content: "先忧后乐体现了责任意识。",
            submitted_at: 1779792000000,
            max_score: "10",
            answer_version: 7,
            pending_short_text_count: 1,
            status: "pending",
          }],
        },
      }));
    }
    if (url === "/api/v1/exam-attempts/900/questions/901/grade" && init?.method === "POST") {
      return new Response(JSON.stringify({ code: 0, message: "ok", data: { graded: true } }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/grading?space_id=301&exam_id=1"]);

  expect(screen.getByRole("heading", { name: "阅卷中心" })).toBeInTheDocument();
  expect(await screen.findByText("张三")).toBeInTheDocument();
  expect(screen.getByText("岳阳楼记思想内涵")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /张三/ })).getByRole("button", { name: "开始阅卷" }));
  await user.clear(screen.getByLabelText("评分"));
  await user.type(screen.getByLabelText("评分"), "8");
  await user.type(screen.getByLabelText("阅卷评语"), "观点完整，表达清楚。");
  await user.click(screen.getByRole("button", { name: "保存阅卷" }));

  expect(screen.getByRole("status", { name: "grading-save-result" })).toHaveTextContent("8 分");
  expect(screen.getByRole("status", { name: "grading-complete-result" })).toHaveTextContent("已完成");
  expect(screen.getByRole("row", { name: /张三/ })).toHaveTextContent("已完成");
  expect(screen.getByText("待阅卷 0")).toBeInTheDocument();
});

test("真实路由渲染阅卷中心时从 session 派生租户和阅卷人", async () => {
  storeSpaceTeacherSession();
  let pendingURL = "";
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    if (url.startsWith("/api/v1/grading/pending")) {
      pendingURL = url;
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [] },
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/grading?space_id=301&exam_id=1"]);

  await waitFor(() => expect(pendingURL).toContain("tenant_id=77"));
  expect(pendingURL).toContain("actor_id=55");
  expect(pendingURL).toContain("actor_role=teacher");
  expect(pendingURL).toContain("space_id=301");
});

test("教师未加入空间时阅卷中心展示受限提示且不请求列表", () => {
  storeTenantTeacherSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/grading?exam_id=1"]);

  expect(screen.getByText("该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷")).toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});
test("教师未加入空间时题库页展示受限提示且不请求列表", () => {
  storeTenantTeacherSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/questions"]);

  expect(screen.getByText("该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷")).toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});


test("真实路由渲染空间管理时从 session 派生租户并请求后端 API", async () => {
  storeTenantAdminSession();
  const requestedURLs: string[] = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    requestedURLs.push(url);
    if (url === "/api/v1/tenant/spaces?tenant_id=77") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 301,
            tenant_id: 77,
            name: "高一 1 班",
            logo_url: "class1.png",
            description: "真实空间",
            members: [{ id: 1, user_id: 88, name: "租户管理员", role: "space_admin", status: "enabled" }],
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      }));
    }
    if (url === "/api/v1/tenant/users?tenant_id=77") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 88,
            tenant_id: 77,
            username: "tenant.admin",
            real_name: "租户管理员",
            avatar_url: "",
            role: "tenant_admin",
            status: "enabled",
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/spaces"]);

  expect(await screen.findByText("高一 1 班")).toBeInTheDocument();
  expect(requestedURLs).toContain("/api/v1/tenant/spaces?tenant_id=77");
  expect(requestedURLs).toContain("/api/v1/tenant/users?tenant_id=77");
  expect(requestedURLs).not.toContain("/api/v1/tenant/spaces?tenant_id=0");
});

test("真实空间成员入口由授权空间列表渲染并读取成员", async () => {
  storeSpaceAdminTeacherSession();
  const requestedURLs: string[] = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    requestedURLs.push(url);
    if (url === "/api/v1/tenant/spaces/301/members?tenant_id=77") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: [{
          id: 1,
          user_id: 55,
          name: "空间管理员",
          role: "space_admin",
          status: "enabled",
        }],
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/space-members"]);

  expect(await screen.findByRole("heading", { name: "空间成员" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /空间成员/ })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /空间管理/ })).not.toBeInTheDocument();
  const memberRow = await screen.findByRole("row", { name: /空间管理员.*55.*启用/ });
  expect(within(memberRow).getByRole("combobox", { name: "修改 空间管理员 的空间身份" })).toHaveValue("space_admin");
  expect(screen.getByText("空间 301")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "创建空间" })).not.toBeInTheDocument();
  expect(requestedURLs).toEqual(["/api/v1/tenant/spaces/301/members?tenant_id=77"]);
});

test("租户管理员不展示独立空间成员入口", () => {
  storeTenantAdminSession();
  const fetchMock = vi.spyOn(globalThis, "fetch");

  renderApp(["/space-members"]);

  expect(screen.getByRole("link", { name: /空间管理/ })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /用户管理/ })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /空间成员/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "空间成员" })).not.toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

test("真实路由渲染用户管理时从 session 派生租户并请求后端 API", async () => {
  storeTenantAdminSession();
  let usersURL = "";
  let spacesURL = "";
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    if (url.startsWith("/api/v1/tenant/users?")) {
      usersURL = url;
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 88,
            tenant_id: 77,
            username: "tenant.admin",
            real_name: "租户管理员",
            avatar_url: "",
            role: "tenant_admin",
            status: "enabled",
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      }));
    }
    if (url.startsWith("/api/v1/tenant/spaces?")) {
      spacesURL = url;
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [], page: 1, page_size: 20, total: 0 },
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/users"]);

  expect(await screen.findByText("tenant.admin")).toBeInTheDocument();
  await waitFor(() => expect(usersURL).toBe("/api/v1/tenant/users?tenant_id=77"));
  await waitFor(() => expect(spacesURL).toBe("/api/v1/tenant/spaces?tenant_id=77"));
});

test("空间管理员直达成绩页时使用当前空间授权身份", async () => {
  storeSpaceAdminTeacherSession();
  let resultsURL = "";
  let exportBody = "";
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url.startsWith("/api/v1/results?")) {
      resultsURL = url;
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [] },
      }));
    }
    if (url === "/api/v1/results/export" && init?.method === "POST") {
      exportBody = String(init.body);
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { file_path: "space-301-scores.csv", file_url: "/api/v1/results/export-files/space-301-scores.csv", row_count: 0 },
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/results?space_id=301&exam_id=1"]);

  expect(screen.getByRole("heading", { name: "成绩" })).toBeInTheDocument();
  await waitFor(() => expect(resultsURL).toContain("actor_role=space_admin"));
  expect(resultsURL).toContain("space_id=301");
  await userEvent.click(screen.getByRole("button", { name: "导出成绩" }));

  await waitFor(() => expect(exportBody).toContain('"actor_role":"space_admin"'));
  expect(exportBody).toContain('"space_id":301');
});

test("成绩页支持发布配置和成绩导出", async () => {
  const user = userEvent.setup();
  storeTenantAdminSession();
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url.startsWith("/api/v1/results?")) {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 1,
            student_name: "张三",
            space_name: "高一 1 班",
            attempt_no: 1,
            objective_score: "2",
            subjective_score: "4.5",
            total_score: "6.5",
            submitted_at: 1779792000000,
            status: "可发布",
          }],
        },
      }));
    }
    if (url === "/api/v1/results/publish-config" && init?.method === "POST") {
      return new Response(JSON.stringify({ code: 0, message: "ok", data: { saved: true } }));
    }
    if (url === "/api/v1/results/export" && init?.method === "POST") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { file_path: "exam-1-scores.csv", file_url: "/api/v1/results/export-files/exam-1-scores.csv?tenant_id=10&exam_id=1", row_count: 1 },
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  renderApp(["/results?exam_id=1"]);

  expect(screen.getByRole("heading", { name: "成绩" })).toBeInTheDocument();
  expect(screen.getByText("成绩发布配置")).toBeInTheDocument();
  expect(await screen.findByText("张三")).toBeInTheDocument();
  expect(screen.getByText("客观题分")).toBeInTheDocument();
  expect(screen.getByText("主观题分")).toBeInTheDocument();

  await user.type(screen.getByLabelText("统一公布时间"), "2026-05-30T10:00");
  await user.click(screen.getByRole("button", { name: "保存发布配置" }));

  expect(screen.getByRole("status", { name: "result-publish-config" })).toHaveTextContent("2026-05-30 10:00");

  await user.click(screen.getByRole("button", { name: "导出成绩" }));

  expect(screen.getByRole("status", { name: "result-export" })).toHaveTextContent("已导出 1 行");
  expect(screen.getByRole("link", { name: "下载导出文件" })).toHaveAttribute("download", "papermind-results.csv");
});
