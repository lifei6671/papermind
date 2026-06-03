const questionImportTemplateRows = [
  ["题型", "题干", "选项", "正确答案", "标准答案", "参考答案", "题目解析", "难度", "标签"],
  ["单选题", "### Markdown 题干示例\n> 阅读下面材料\n\n- 主动作为\n- 服务他人\n\n下列哪一项体现责任担当？", "A.责任担当|B.及时行乐|C.功名至上", "A", "", "", "责任担当体现公共意识和主动作为。", "中等", "语文,阅读理解"],
  ["多选题", "多选题示例：下列属于网络安全措施的是？", "A.设置强密码|B.定期备份|C.共享账号|D.安装补丁", "A,B,D", "", "", "强密码、备份和补丁都能降低安全风险。", "中等", "信息技术,网络安全"],
  ["判断题", "判断题示例：纸质书属于电子出版物。", "", "", "错误", "", "纸质书不是电子出版物。", "简单", "出版,常识"],
  ["填空题", "填空题示例：HTTP 默认端口是____，HTTPS 默认端口是____。", "", "", "80|443", "", "HTTP 默认端口为 80，HTTPS 默认端口为 443。", "简单", "信息技术,网络基础"],
  ["简答题", "简答题示例：简述零信任架构的核心思想。", "", "", "", "持续验证身份、最小权限访问，并默认不信任内外部网络。", "回答需覆盖身份验证、访问控制和持续校验。", "困难", "信息技术,网络安全"],
];

export const questionImportTemplateFileName = "question-import-template.csv";

export const questionImportTemplateHref = `data:text/csv;charset=utf-8,${encodeURIComponent(`\uFEFF${toCSV(questionImportTemplateRows)}`)}`;

function toCSV(rows: string[][]) {
  return rows.map((row) => row.map(escapeCSVCell).join(",")).join("\n");
}

function escapeCSVCell(value: string) {
  if (/[",\n]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}
