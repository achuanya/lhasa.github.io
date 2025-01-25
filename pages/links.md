---
layout: page
permalink: /links.html
title: 订阅
---
<section class="page-content">
  <section class="post-list">
    {% assign rss_data = site.data.rss_data %}
    {% for post in rss_data %}
      <article class="post-item">
        <i class="post-item-thumb" style="background-image:url({{ post.avatar }})"></i>
        <section class="post-item-summary">
          <h3 class="post-item-title">
            <a class="post-item-link" href="{{ post.link }}" title="{{ post.title }}" target="_blank">{{ post.title }}</a>
          </h3>
          <time class="post-item-date timeago" datetime="{{ post.date | date_to_xmlschema }}">{{ post.date | date: "%Y年%m月%d日" }}</time>
          <address class="post-item-date links-name">{{ post.name }}</address>
        </section>
      </article>
    {% endfor %}
  </section>
</section>
{% include wechat.html %}