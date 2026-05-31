import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { useSession } from "./session-context";
import { SessionProvider } from "./session";

afterEach(() => {
  window.localStorage.clear();
  vi.restoreAllMocks();
});

function SessionProbe() {
  const { session, signIn, signOut } = useSession();

  return (
    <div>
      <span>{session ? session.user.displayName : "未登录"}</span>
      <button
        onClick={() =>
          signIn({
            user: { displayName: "李老师", role: "teacher", userID: 9 },
          })
        }
      >
        登录
      </button>
      <button onClick={signOut}>退出</button>
    </div>
  );
}

test("登录态写入本地存储并在 Provider 重建后恢复", async () => {
  const user = userEvent.setup();
  const { unmount } = render(
    <SessionProvider>
      <SessionProbe />
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "登录" }));

  expect(screen.getByText("李老师")).toBeInTheDocument();
  expect(window.localStorage.getItem("papermind.session.v1")).toContain("李老师");

  unmount();

  render(
    <SessionProvider>
      <SessionProbe />
    </SessionProvider>,
  );

  expect(screen.getByText("李老师")).toBeInTheDocument();
});

test("退出登录会清理本地存储", async () => {
  const user = userEvent.setup();
  const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ code: 0, message: "ok", data: { logged_out: true } })));
  render(
    <SessionProvider>
      <SessionProbe />
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "登录" }));
  await user.click(screen.getByRole("button", { name: "退出" }));

  expect(screen.getByText("未登录")).toBeInTheDocument();
  expect(window.localStorage.getItem("papermind.session.v1")).toBeNull();
  await waitFor(() => expect(fetcher).toHaveBeenCalledWith(
    "/api/v1/auth/logout",
    expect.objectContaining({ method: "POST" }),
  ));
});
