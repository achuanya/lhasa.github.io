---
layout: post
title: 腾讯云COS文件跨域
date: 2024-02-05 17:33:06 +0800
category: tech
thumb: ARTICLEPICTURES_PATH/Tencent-cloud-COS-file-cross-domains.png
tags: [COS, 跨域, 取子集]
---

今天换博客主要文字了，"仓耳今楷",字体更美观更适合阅读。但是过程中遇到点问题

```css
@font-face {
    font-family: 仓耳今楷01-W04;
    src: url("https://api.lhasa.icu/assets/font/tsanger01W04.ttf")  format("truetype");
}
```

这段CSS写的是没有问题的，但是不生效，控制台报错跨域

* has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header is present on the requested resource.

腾讯云COS跨域访问CORS配置如下：

![腾讯云COS跨域访问CORS设置][1]{:.small}

配置好后又遇到麻烦了，字体太大了，一个字体文件17.9M！网站都脱垮了

![网站被拖垮了][2]{:.small}

这里做一下处理，取子集压缩文字，需要用到 [FontSmaller][3] 和 [现代汉语常用3500汉字][4]

取子集压缩之后字体文件大小为1.94M

![取子集压缩后的效果][5]{:.small}


[1]: {{ site.ARTICLEPICTURES_PATH }}/Tencent%20cloud%20cos%20cross-domain%20configuration.jpg
[2]: {{ site.ARTICLEPICTURES_PATH }}/Textloading.jpg
[3]: https://fontsmaller.github.io/
[4]: https://lhasa.icu/assets/%E7%8E%B0%E4%BB%A3%E6%B1%89%E8%AF%AD%E5%B8%B8%E7%94%A83500%E6%B1%89%E5%AD%97.txt
[5]: {{ site.ARTICLEPICTURES_PATH }}/Textloading2.png
