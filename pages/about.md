---
layout: page
permalink: /about.html
title: 关于
tags: [关于, 博客, 游钓四方, blog]
---

<a href="https://996.icu" target="_blank">
    <img src="https://cos.lhasa.icu/svg/link-996.icu-red.svg" alt="996.icu" />
</a>

* Blog：<a href="https://lhasa.icu" target="_blank">https://lhasa.icu</a>
* Atom：<a href="https://lhasa.icu/atom.xml" target="_blank">https://lhasa.icu/atom.xml</a>
* Rss：<a href="https://lhasa.icu/rss.xml" target="_blank">https://lhasa.icu/rss.xml</a>

自 2018 年 8 月 31 日起，本站已运行 <span id="days"></span> 天，截至到今天，共写了博文 {{ site.posts.size }} 篇，计 {% assign count = 0 %}{% for post in site.posts %}{% assign single_count = post.content | strip_html | strip_newlines | remove: ' ' | size %}{% assign count = count | plus: single_count %}{% endfor %}{% if count > 10000 %}{{ count | divided_by: 10000 }} 万 {{ count | modulo: 10000 }}{% else %}{{ count }}{% endif %} 字。

鄙人博客采用:<a href="https://creativecommons.org/licenses/by-nc-nd/3.0/deed.zh-hans" target="_blank">CC BY-NC-ND 4.0</a>，转载请务必注明出处，谢谢。

内容系本人学习、研究和总结，如有雷同，实属荣幸！

## 博主

千禧年小孩，长途骑行小学生、野钓路亚、振出并继、古典乐、摇滚、布鲁斯、茶叶爱好者

![游钓四方的骑行照]({{ site.STYLEPICTURES_PATH}}/my-photo.jpg_640 "游钓四方的骑行照")

## 博客进程

* 2018-08-30 Fork孔令贤的Jekyll模板，开始接触个人博客
* 2018-08-31 博客由Github Pages托管，起名：阿川的个人博客
* 2018-10-16 Dynadot购入域名achuan.io，该域名于2023-10-16到期
* 2020-10-06 加入Forever Blog十年之约
* 2024-01-22 因代码历史遗留问题，舍弃原有博客。Fork Fooleap Blog
* 2024-01-23 Tencent Cloud购入域名lhasa.icu，博客更名：游钓四方的博客
* 2024-01-31 CSS、JS和Images转移到Tencent Cloud COS
* 2024-01-31 完成域名备案，并由Tencent Cloud CDN进行全球加速
* 2024-02-06 引入Disqus评论系统，由洛杉矶VPS，Ningx反向代理
* 2024-02-11 CSS和JS由WebPack打包，字体进行了分包处理
* 2024-07-22 Tencent Cloud COS再套一层CDN进行境内加速

{% include wechat.html %}

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