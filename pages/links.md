---
layout: page
permalink: /links.html
title: 订阅
---
<section class="page-content">
  <section class="post-list">
  </section>
</section>
{% include wechat.html %}

<script>
  document.addEventListener("DOMContentLoaded", function() {
    fetch('https://cos.lhasa.icu/data/rss_data.json')
      .then(response => response.json())
      .then(rss_data => {
        const container = document.querySelector('.post-list');
        rss_data.forEach(post => {
          const article = document.createElement('article');
          article.classList.add('post-item');
          
          article.innerHTML = `
            <i class="post-item-thumb" style="background-image:url(${post.avatar})"></i>
            <section class="post-item-summary">
              <h3 class="post-item-title">
                <a class="post-item-link" href="${post.link}" title="${post.title}" target="_blank">${post.title}</a>
              </h3>
              <time class="post-item-date timeago" datetime="${post.date}">${post.date}</time>
              <address class="post-item-date links-name">${post.name}</address>
            </section>
          `;
          
          container.appendChild(article);
        });
      })
      .catch(error => console.error('Error loading RSS data:', error));
  });
</script>