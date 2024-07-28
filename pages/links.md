---
layout: page
permalink: /links.html
title: 链接
---
<div class="container">
  {% assign rss_data = site.data.rss_data %}
  {% for post in rss_data %}
  <div class="card">
      <h1>
          <a href="{{ post.domainName }}" target="_blank">{{ post.name }}</a>
      </h1>
      <time>{{ post.date | date: "%B %d, %Y" }}</time>
      <a href="{{ post.link }}" target="_blank">{{ post.title }}</a>
  </div>
  {% endfor %}
</div>

