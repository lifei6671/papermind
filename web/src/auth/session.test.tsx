import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";
import { useSession } from "./session-context";
import { SessionProvider } from "./session";

afterEach(() => {
  window.localStorage.clear();
});

function SessionProbe() {
  const { session, signIn, signOut } = useSession();

  return (
    <div>
      <span>{session ? session.user.displayName : "未登录"}</span>
      <button
        onClick={() =>
          signIn({
            accessToken: "access-token",
            refreshToken: "refresh-token",
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
  expect(window.localStorage.getItem("papermind.session.v1")).toContain("access-token");

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
  render(
    <SessionProvider>
      <SessionProbe />
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "登录" }));
  await user.click(screen.getByRole("button", { name: "退出" }));

  expect(screen.getByText("未登录")).toBeInTheDocument();
  expect(window.localStorage.getItem("papermind.session.v1")).toBeNull();
});
