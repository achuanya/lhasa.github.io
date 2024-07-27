---
layout: page
permalink: /links.html
title: 链接
---
<p class="center-text">小小页面，虚拟世界。虽未曾谋面，已心生敬仰</p>
<div class="container">
  {% assign rss_data = site.data.rss_data %}
  {% for post in rss_data %}
  <div class="card">
      <h1>
          <a href="https://lhasa.icu" target="_blank">{{ post.name }}</a>
      </h1>
      <time>{{ post.date | date: "%B %d, %Y" }}</time>
      <a href="{{ post.link }}" target="_blank">{{ post.title }}</a>
  </div>
  {% endfor %}
</div>

<div class="comment-guestbook">
  <div id="comment"></div>
</div>

