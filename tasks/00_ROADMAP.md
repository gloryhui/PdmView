# 开发路线图与依赖

```text
P0 项目勘察与环境验证
  |
  v
P1 MyGo Native Canvas 原型 -----> P2 性能与交互原型
  |                                  |
  +----------------------------------+
                 |
                 v
P3 PDM 格式研究与解析器
                 |
                 v
P4 Domain Model / Diagram 映射
                 |
                 v
P5 Graph Engine / 空间索引
                 |
                 v
P6 Renderer / ER 画布
                 |
                 v
P7 详情面板 / 搜索 / 复制
                 |
                 v
P8 MySQL SQL Generator
                 |
                 v
P9 SQL 预览 / 导出 / 集成测试
                 |
                 v
P10 跨平台验收 / 发布
```

## 阶段原则

- P0～P2 必须先完成，确认 Native UI 和大型画布技术路线可行。
- P3 可与 P1/P2 的部分非 UI 工作并行，但必须使用真实样本验证。
- P4 完成后冻结核心 Domain Model，再大规模并行。
- P5/P6 必须依赖统一模型和 Diagram 映射。
- P7 依赖 PDM 模型和基础 UI。
- P8/P9 与画布 UI 可在接口冻结后并行。
- P10 必须等所有 P0 功能集成完成。

## 完成定义

每个任务都必须满足：
- 代码已落地。
- 测试已运行。
- 文档已更新。
- 主 Agent 审查通过。
- 任务状态表更新。
- 不存在未披露的占位实现。
