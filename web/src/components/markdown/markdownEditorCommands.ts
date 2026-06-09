import { createElement } from "react";
import type { ICommand } from "@uiw/react-md-editor/commands";

const markdownCommandTitles: Record<string, string> = {
  bold: "加粗（Ctrl+B）",
  italic: "斜体（Ctrl+I）",
  strikethrough: "删除线（Ctrl+Shift+X）",
  hr: "插入分隔线（Ctrl+H）",
  title: "标题",
  heading1: "一级标题（Ctrl+1）",
  heading2: "二级标题（Ctrl+2）",
  heading3: "三级标题（Ctrl+3）",
  heading4: "四级标题（Ctrl+4）",
  heading5: "五级标题（Ctrl+5）",
  heading6: "六级标题（Ctrl+6）",
  link: "插入链接（Ctrl+L）",
  quote: "插入引用（Ctrl+Q）",
  code: "行内代码（Ctrl+J）",
  codeBlock: "代码块（Ctrl+Shift+J）",
  comment: "插入注释（Ctrl+/）",
  image: "插入图片（Ctrl+K）",
  table: "插入表格",
  "unordered-list": "无序列表（Ctrl+Shift+U）",
  "ordered-list": "有序列表（Ctrl+Shift+O）",
  "checked-list": "任务列表（Ctrl+Shift+C）",
  help: "打开语法帮助",
  issue: "插入议题编号",
  edit: "编辑源码（Ctrl+7）",
  live: "实时预览（Ctrl+8）",
  preview: "预览（Ctrl+9）",
  fullscreen: "全屏（Ctrl+0）",
};

const headingIconLabels: Record<string, string> = {
  heading1: "一级标题",
  heading2: "二级标题",
  heading3: "三级标题",
  heading4: "四级标题",
  heading5: "五级标题",
  heading6: "六级标题",
};

const headingIconFontSizes: Record<string, number> = {
  heading1: 18,
  heading2: 16,
  heading3: 15,
  heading4: 14,
  heading5: 12,
  heading6: 12,
};

export function localizeMarkdownEditorCommand(command: ICommand): ICommand {
  const localizedChildren = Array.isArray(command.children)
    ? command.children.map(localizeMarkdownEditorCommand)
    : undefined;
  const title = readMarkdownCommandTitle(command);
  if (!title && localizedChildren === undefined) {
    return command;
  }

  const nextCommand = { ...command } as ICommand;
  if (localizedChildren !== undefined) {
    Object.assign(nextCommand, { children: localizedChildren });
  }

  if (title) {
    nextCommand.buttonProps = {
      ...(command.buttonProps ?? {}),
      "aria-label": title,
      title,
    };
  }

  const headingIconLabel = command.name ? headingIconLabels[command.name] : undefined;
  if (headingIconLabel) {
    nextCommand.icon = createElement(
      "div",
      {
        style: {
          fontSize: headingIconFontSizes[command.name ?? ""] ?? 12,
          textAlign: "left",
        },
      },
      headingIconLabel,
    );
  }

  return nextCommand;
}

function readMarkdownCommandTitle(command: ICommand) {
  if (command.keyCommand === "preview" && typeof command.value === "string") {
    return markdownCommandTitles[command.value];
  }
  if (command.name && markdownCommandTitles[command.name]) {
    return markdownCommandTitles[command.name];
  }
  if (command.keyCommand && markdownCommandTitles[command.keyCommand]) {
    return markdownCommandTitles[command.keyCommand];
  }
  return undefined;
}
