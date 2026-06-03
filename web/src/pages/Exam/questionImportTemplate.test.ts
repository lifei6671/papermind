import { expect, test } from "vitest";
import { questionImportTemplateHref } from "./questionImportTemplate";

test("题目导入模板使用中文表头并覆盖全部题型示例", () => {
  const csvText = decodeURIComponent(questionImportTemplateHref.replace("data:text/csv;charset=utf-8,", ""));

  expect(csvText).toContain("\uFEFF题型,题干,选项,正确答案,标准答案,参考答案,题目解析,难度,标签");
  expect(csvText).toContain("### Markdown 题干示例");
  expect(csvText).toContain("多选题示例");
  expect(csvText).toContain("判断题示例");
  expect(csvText).toContain("填空题示例");
  expect(csvText).toContain("简答题示例");
});
