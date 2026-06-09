import type { ThemeConfig } from "antd/es/config-provider";

export const paperMindAntTheme: ThemeConfig = {
  token: {
    colorPrimary: "#dc7556",
    colorInfo: "#64748b",
    colorSuccess: "#00a842",
    colorWarning: "#ff6b2a",
    colorError: "#d54b3d",
    colorText: "#171c28",
    colorTextSecondary: "#4e5868",
    colorBorder: "#e4e1d8",
    colorBgContainer: "#ffffff",
    borderRadius: 8,
    fontFamily: "Noto Sans SC, Noto Sans SC Fallback, Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif",
    fontSize: 13,
  },
  components: {
    Button: {
      borderRadius: 8,
      controlHeight: 32,
      fontWeight: 700,
    },
  },
};
