---
layout: post
title: insertRule兼容性
date: 2024-08-07 14:26:01 +0800
category: tech
thumb: ARTICLEPICTURES_PATH/Form-automation-plugin.png
tags: [插件, chrome, 自动化, 表单]
---

insertRule是CSS动态添加新规则的方法，不需要直接修改页面的CSS，它在JS中可以直接动态生成和添加样式。

## 参数

* rule（字符串类型）：

这个参数是一个包含 CSS 规则的字符串。例如："body { background-color: blue; }".
这个字符串应该遵循标准的 CSS 语法。

* index（整数类型）：

这是可选参数，表示你希望将规则插入到样式表中的位置（以规则在样式表中的顺序为基础）。

index 为 0 表示将规则插入到样式表的开头，如果省略此参数或提供的 index 超出范围，规则将被添加到样式表的末尾。

* 返回值

成功插入规则时，insertRule 方法返回新插入规则的索引（整数）。

### 示例

```js
// 获取文档的第一个样式表
var sheet = document.styleSheets[0];

// 在样式表末尾插入一个新规则，改变所有段落元素的颜色为蓝色
sheet.insertRule('p { color: blue; }', sheet.cssRules.length);
```

## 兼容性

| 浏览器/平台           | insertRule() 支持版本 | `index` 参数为可选参数的支持版本 |
|-----------------------|-----------------------|-----------------------------------|
| **Chrome**            | 1                     | 55                                |
| **Edge**              | 12                    | 15                                |
| **Firefox**           | 1                     | 18                                |
| **Opera**             | 12.1                  | 55                                |
| **Safari**            | 1                     | 14                                |
| **Chrome Android**    | 18                    | 55                                |
| **Firefox for Android** | 4                    | 18                              |
| **Opera Android**     | 12.1                  | 55                                |
| **Safari on iOS**     | 1                     | 14                                |
| **Samsung Internet**  | 1.0                   | 1.0                               |
| **WebView Android**   | 4.4                   | 4.4                               |

## BUG

按照mdn web docs基于的规则来说，一般会有四种异常情况：

SyntaxError：

当 rule 参数的语法无效时（例如缺少大括号或使用了不正确的 CSS 属性名），将会抛出此异常。
IndexSizeError：

如果指定的 index 超出了当前样式表中规则的数量范围，会抛出此异常。
HierarchyRequestError：

如果你试图在不允许的位置插入某些类型的规则，比如试图将规则插入到 @keyframes 内，或插入 @namespace 规则到非首个位置。
NoModificationAllowedError：

如果样式表是只读的，或者由于跨域安全限制，JavaScript 无法访问样式表（比如样式表来自不同域），将会抛出此异常。




这个博客是[Fooleap](https://blog.fooleap.org)写的，
