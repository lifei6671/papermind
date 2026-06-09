import ConfigProvider from "antd/es/config-provider";
import zhCN from "antd/es/locale/zh_CN";
import dayjs from "dayjs";
import "dayjs/locale/zh-cn";
import type { ReactNode } from "react";
import { SessionProvider } from "../auth/session";
import { paperMindAntTheme } from "./antd-theme";
import { FeedbackProvider } from "./feedback";

dayjs.locale("zh-cn");

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <ConfigProvider locale={zhCN} theme={paperMindAntTheme}>
      <SessionProvider>
        <FeedbackProvider>{children}</FeedbackProvider>
      </SessionProvider>
    </ConfigProvider>
  );
}
