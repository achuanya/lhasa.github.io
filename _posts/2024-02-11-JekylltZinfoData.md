---
layout: post
title: 解决Jekyll时区数据源
date: 2024-02-11 23:07:20 +0800
category: tech
thumb: ARTICLEPICTURES_PATH/JekylltZinfoData.png
tags: [jekyll, tzinfo-data]
---

由于Jekyll默认使用UTC时区，导致博客更新时间不准确。这里需要写入上海时间：timezone: Asia/Shanghai，但是我在本地调试时需要在配置内注释掉，不然就会报错

* jekyll 3.9.3 | Error:  No source of timezone data could be found.
Please refer to https://tzinfo.github.io/datasourcenotfound for help resolving this error.

上传到仓库 Github pages 不会出现这样的问题。老是注释调试挺麻烦的，Google搜出来的解决方案都是瞎扯淡，也不知道都是哪复制粘贴就发出来的。

```bash
gem install tzinfo-data
```

Gemfile 直接指定版本
```bash
gem 'tzinfo-data', '>= 1.2021a'
```

写入配置 timezone: Asia/Shanghai，确保调试的电脑时区也正常，开始运行
```bash
bundle exec jekyll serve
```