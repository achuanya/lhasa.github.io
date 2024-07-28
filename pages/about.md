---
layout: page
permalink: /about.html
title: 关于
tags: [关于, 博客, 游钓四方, blog]
---

* 博客网址：[https://lhasa.icu](https://lhasa.icu)
* Atom 订阅：[https://lhasa.icu/atom.xml](/atom.xml)

<iframe frameborder="no" border="0" marginwidth="0" marginheight="0" width=298 height=52 src="//music.163.com/outchain/player?type=2&id=22603037&auto=1&height=32"></iframe>

请使用 Firefox、Chrome 等现代浏览器浏览本博客，以免因为兼容性影响阅读体验。

自 2018 年 8 月 31 日起，本站已运行 <span id="days"></span> 天，截至到今天，共写了博文 {{ site.posts.size }} 篇，计 {% assign count = 0 %}{% for post in site.posts %}{% assign single_count = post.content | strip_html | strip_newlines | remove: ' ' | size %}{% assign count = count | plus: single_count %}{% endfor %}{% if count > 10000 %}{{ count | divided_by: 10000 }} 万 {{ count | modulo: 10000 }}{% else %}{{ count }}{% endif %} 字。

鄙人博客采用:[CC BY-NC-ND 4.0][1]，转载请务必注明出处，谢谢。

内容系本人学习、研究和总结，如有雷同，实属荣幸！

## 博主

![游钓四方的骑行照]({{ site.STYLEPICTURES_PATH}}/my-photo.jpg_640 "游钓四方的骑行照")

长途骑行小学生、野钓路亚、振出并继、古典乐、茶叶爱好者

- Email: <haibao1027@gmail.com>
- Github：[achuanya][2]
- 微信公众号：游钓四方的博客

![游钓四方的微信公众号]({{ site.STYLEPICTURES_PATH}}/WechatPublicAccount.jpg "生活中从不缺少美，而是缺少发现美的眼睛")

## 博客进程

* 2018-08-30 Fork 云计算大佬 孔令贤的 Jekyll 模板，开始接触个人博客
* 2018-08-31 由 Github Pages 托管，起名：阿川的个人博客
* 2018-10-16 Dynadot 购入 achuan.io，于 2023-10-16 到期
* 2020-10-06 加入 Forever Blog 十年之约
* 2024-01-23 腾讯云购入 lhasa.icu，博客改名为：游钓四方的博客
* 2024-01-22 因代码历史遗留问题，舍弃原有博客，Fork Fooleap Blog
* 2024-01-31 全站静态资源走腾讯 对象储存 COS
* 2024-01-31 域名备案完成，由腾讯云 内容分发网络 CDN 全球加速
* 2024-02-06 由 Ningx 反向代理引入 Disqus 评论系统
* 2024-02-11 CSS、JS 由 webPack 打包，字体做分包处理

[1]: https://creativecommons.org/licenses/by-nc-nd/4.0/deed.zh-hans
[2]: https://github.com/achuanya

<script>
    var days = 0, daysMax = Math.floor((Date.now() / 1000 - {{ "2018-08-31" | date: "%s" }}) / (60 * 60 * 24));
    (function daysCount(){
        if(days > daysMax){
            document.getElementById('days').innerHTML = daysMax;
            return;
        } else {
            document.getElementById('days').innerHTML = days;
            days += 10;
            setTimeout(daysCount, 1); 
        }
    })();
</script>
