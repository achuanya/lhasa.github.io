import iDisqus from 'disqus-php-api';
import './sass/main.scss';

/**
 * [timeAgo 时间格式化函数]
 * 
 * 每分钟更新一次页面中 class="timeago" 的元素显示成“几秒/几小时/几天前”等。
 * 原理：对比当前时间与元素 datetime 属性值，动态计算差值
 */
function timeAgo() {
  // 中文显示模板，可根据需求修改
  const templates = {
    prefix: "",
    suffix: "前",
    seconds: "几秒",
    minute: "1分钟",
    minutes: "%d分钟",
    hour: "1小时",
    hours: "%d小时",
    day: "1天",
    days: "%d天",
    month: "1个月",
    months: "%d个月",
    year: "1年",
    years: "%d年"
  };

  /**
   * [template 用于获取不同时间段对应的字符串]
   * @param {String} t - 模板key
   * @param {Number} n - 时间数值
   */
  const template = (t, n) => {
    return templates[t] && templates[t].replace(/%d/i, Math.abs(Math.round(n)));
  };

  /**
   * [timer 核心函数，传入时间字符串，返回格式化的 xx前]
   * @param {String} time - 元素 datetime 的内容
   */
  const timer = (time) => {
    if (!time) return;
    // 移除毫秒
    time = time.replace(/\.\d+/, "");
    // 统一时间格式兼容
    time = time.replace(/-/, "/").replace(/-/, "/");
    time = time.replace(/T/, " ").replace(/Z/, " UTC");
    // -04:00 -> -0400 兼容时区写法
    time = time.replace(/([\+\-]\d\d)\:?(\d\d)/, " $1$2");
    time = new Date(time * 1000 || time);

    const now = new Date();
    const seconds = ((now.getTime() - time) * 0.001) >> 0;  // 或使用 Math.floor()
    const minutes = seconds / 60;
    const hours = minutes / 60;
    const days = hours / 24;
    const years = days / 365;

    return templates.prefix + (
      seconds < 45 && template('seconds', seconds) ||
      seconds < 90 && template('minute', 1) ||
      minutes < 45 && template('minutes', minutes) ||
      minutes < 90 && template('hour', 1) ||
      hours < 24 && template('hours', hours) ||
      hours < 42 && template('day', 1) ||
      days < 30 && template('days', days) ||
      days < 45 && template('month', 1) ||
      days < 365 && template('months', days / 30) ||
      years < 1.5 && template('year', 1) ||
      template('years', years)
    ) + templates.suffix;
  };

  // 找到页面中所有时间节点，批量更新
  const elements = document.getElementsByClassName('timeago');
  for (let i in elements) {
    const elem = elements[i];
    if (typeof elem === 'object') {
      elem.innerHTML = timer(elem.getAttribute('datetime'));
    }
  }
  // 每分钟更新一次
  setTimeout(timeAgo, 60000);
}

/**
 * [getQuery 获取URL查询参数]
 * @param {String} variable - 参数名
 * @returns {String|Boolean} - 返回对应的查询值，若无则返回false
 */
function getQuery(variable) {
  const query = window.location.search.substring(1);
  const vars = query.split("&");
  for (let i = 0; i < vars.length; i++) {
    let pair = vars[i].split("=");
    if (pair[0] === variable) {
      return pair[1];
    }
  }
  return false;
}

/**
 * 页面关闭时取消菜单选中状态
 */
window.addEventListener('beforeunload', function () {
  document.getElementById('menu').checked = false;
});

/**
 * [renderPostItem 生成单个文章元素HTML，针对不同分类做差异处理]
 * @param {Object} item      - 单条文章数据
 * @param {String} category  - 分类：life/cycling/tech
 * @returns {String}         - 拼接好的HTML字符串
 */
function renderPostItem(item, category) {
  // 针对 life/cycling 时使用 item.image，tech 时使用 item.thumb
  const isTech = category === 'tech';
  const imageUrl = isTech ? item.thumb : item.image;
  // 判断是否需要拼接特定的缩略图后缀
  const bgUrl = (!isTech && imageUrl.indexOf('svg') === -1)
    ? (imageUrl + '?imageView2/1/w/400/h/266')
    : imageUrl;

  // 文章标题中是否带有图片数量，比如 [30P]
  const imageCount = item.images > 30 && item.category === category
    ? '[' + item.images + 'P]'
    : '';

  // life/cycling 与 tech 在评论链接标记有所差异
  if (!isTech) {
    // life.html/cycling.html 的文章结构
    return `
      <article class="post-item">
        <i class="post-item-thumb" data-src="${imageUrl}" style="background-image:url(${bgUrl})"></i>
        <section class="post-item-summary">
          <h3 class="post-item-title">
            <a class="post-item-link" href="${item.url}" title="${item.title}">
              ${item.title}${imageCount}
            </a>
          </h3>
        </section>
        <section class="post-item-footer">
          <time class="post-item-date timeago" datetime="${item.date}"></time>
          <a class="post-item-cmt" title="查看评论" href="${item.url}#comment">
            <span data-disqus-url="${item.url}"></span><span>条评论</span>
          </a>
        </section>
      </article>
    `;
  } else {
    // tech.html 的文章结构
    return `
      <article class="post-item">
        <i class="post-item-thumb" data-src="${imageUrl}" style="background-image:url(${imageUrl})"></i>
        <section class="post-item-summary">
          <h3 class="post-item-title">
            <a class="post-item-link" href="${item.url}" title="${item.title}">
              ${item.title}${imageCount}
            </a>
          </h3>
          <time class="post-item-date timeago" datetime="${item.date}"></time>
        </section>
        <a class="post-item-comment" title="查看评论" data-disqus-url="${item.url}" href="${item.url}#comment"></a>
      </article>
    `;
  }
}

/**
 * [renderPagination 分页HTML]
 * @param {Number} curPage - 当前页码
 * @param {Number} total   - 总页数
 * @param {String} baseUrl - 分类页链接，如 /life.html
 * @returns {String}       - 拼接好的分页HTML
 */
function renderPagination(curPage, total, baseUrl) {
  // 计算上一页 / 下一页
  let prev = curPage > 1 ? curPage - 1 : 0;
  let next = curPage < total ? curPage + 1 : 0;

  let prevLink = prev
    ? `<a class="pagination-item-link" href="${baseUrl}?page=${prev}" data-page="${prev}">较新文章 &raquo;</a>`
    : '';
  let nextLink = next
    ? `<a class="pagination-item-link" href="${baseUrl}?page=${next}" data-page="${next}">&laquo; 较旧文章</a>`
    : '';

  return `
    <ul class="pagination-list">
      <li class="pagination-item">${nextLink}</li>
      <li class="pagination-item">${curPage} / ${total}</li>
      <li class="pagination-item">${prevLink}</li>
    </ul>
  `;
}

/**
 * [turn 翻页与渲染函数]
 * - 仅在匹配到 life.html / cycling.html / tech.html 时被触发
 * - 根据 pageNum 生成相应的文章列表和分页控件
 * @param {Number} pageNum - 当前页码
 * @param {Array} posts    - 该分类下的所有文章数组
 * @param {String} category - 分类：life / cycling / tech
 * @param {Object} disq    - iDisqus实例
 */
function turn(pageNum, posts, category, disq) {
  let pageSize = 10;  // 默认每页10篇，tech.html

  // 根据分类自定义页面显示数以及一些样式等
  let catName;
  let postClass;

  switch (category) {
    case 'life':
      catName = '生活';
      pageSize = 12;
      postClass = 'post-life';
      break;
    case 'cycling':
      catName = '骑行';
      pageSize = 6;
      postClass = 'post-cycling';
      break;
    case 'tech':
      catName = '技术';
      postClass = 'post-tech';
      break;
    default:
      // 若不在上述范围，可根据需求自定义
      catName = '分类';
      postClass = 'post-tech';
      break;
  }

  // 计算文章起止位置
  const totalPosts = posts.length;
  const first = (pageNum - 1) * pageSize;
  const last = totalPosts > pageNum * pageSize ? pageNum * pageSize : totalPosts;
  let html = '';

  // 拼接文章列表HTML
  for (let i = first; i < last; i++) {
    html += renderPostItem(posts[i], category);
  }

  // 计算总页数
  const totalPage = Math.ceil(totalPosts / pageSize);
  // 生成分页HTML
  const paginationHTML = renderPagination(pageNum, totalPage, window.page.url);

  // 插入到页面
  const listElem = document.querySelector('.post-list');
  if (listElem) {
    listElem.classList.add(postClass);
    listElem.innerHTML = html;
  }

  // 更新分页
  const paginationElem = document.querySelector('.pagination');
  if (paginationElem) {
    paginationElem.innerHTML = paginationHTML;
  }

  // 时间友好化/评论数更新
  timeAgo();
  disq.count();

  // 点击分页链接时，无刷新加载
  const link = document.getElementsByClassName('pagination-item-link');
  for (let i = 0; i < link.length; i++) {
    link[i].addEventListener('click', function (e) {
      e.preventDefault();
      const p = parseInt(e.currentTarget.dataset.page);
      turn(p, posts, category, disq);
    });
  }

  // 更新标题与浏览器历史记录
  const title = pageNum === 1
    ? `${catName} | ${site.title}`
    : `${catName}：第${pageNum}页 | ${site.title}`;
  const url = pageNum === 1
    ? window.page.url
    : `${window.page.url}?page=${pageNum}`;

  document.title = title;
  history.replaceState({ "title": title, "url": url }, title, url);

  // 统计
  if (site.home === location.origin && window.parent === window) {
    _hmt.push(['_trackPageview', url]);
  }
}

// DOMContentLoaded 主体逻辑
document.addEventListener('DOMContentLoaded', function () {
  /********
   * 初始化 Disqus
   ********/
  const disq = new iDisqus('comment', {
    forum: site.forum,
    site: site.home,
    api: site.disqus_api,
    title: page.title,
    url: page.url,
    mode: 2,
    timeout: 3000,
    slug: page.url.slice(1).split('.')[0],
    init: true,
    toggle: 'comment-toggle',
    sort: 'newest',
    emoji_path: site.api + '/emoji/unicode/',
  });
  disq.count();

  // 启动TimeAgo
  timeAgo();

  /********
   * 进度条 - 计算当年进度
   ********/
  const curYear = new Date().getFullYear();
  const startYear = Date.parse('01 Jan ' + curYear + ' 00:00:00');
  const endYear = Date.parse('31 Dec ' + curYear + ' 23:59:59');
  const yearProgress = ((Date.now() - startYear) / (endYear - startYear)) * 100;
  const widthProgress = yearProgress.toFixed(2) + '%';

  // 创建style元素以设置伪元素宽度
  const style = document.createElement('style');
  style.type = 'text/css';

  // IE兼容写法
  const cssStr = `.page-header .page-title:before { width: ${widthProgress}; }
                  .page-header .page-title:after { left: ${widthProgress}; content: "${parseInt(yearProgress)}%"; }`;
  if (style.styleSheet) {
    style.styleSheet.cssText = cssStr;
  } else {
    style.appendChild(document.createTextNode(cssStr));
  }
  document.head.appendChild(style);

  /********
   * 针对文章页面处理图片
   ********/
  if (page.layout === 'post') {
    const imageArr = document.querySelectorAll('.post-content img[data-src]:not([class="emoji"])');
    const image = {
      src: [],
      thumb: [],
      title: [],
      coord: []
    };

    // 收集图片信息
    for (let i = 0; i < imageArr.length; i++) {
      image.thumb[i] = imageArr[i].src;
      image.src[i] = imageArr[i].dataset.src;
    }

    // 过滤出 jpg 图片（可按需或保留）
    image.jpg = image.src.filter(function (item) {
      return item.indexOf('.jpg') > -1 && new RegExp(site.img, 'i').test(item);
    });

    // 为每张图片添加标题和包裹
    [].forEach.call(imageArr, function (item, i) {
      image.title[i] = item.title || item.parentElement.textContent.trim() || item.alt;
      item.title = image.title[i];
      item.classList.add('post-image');
      item.setAttribute('data-index', i);

      // figure 包裹
      item.parentElement.outerHTML = item.parentElement.outerHTML
        .replace('<p>', `<figure class="post-figure" data-index=${i}>`)
        .replace('<p>', `<figure class="post-figure" data-index=${i}>`)
        .replace('</p>', '</figure>')
        .replace(item.parentElement.textContent, '');

      // 若图片来源符合站点图床，再插入图片描述
      const imgdom = document.querySelector('.post-image[data-src="' + image.src[i] + '"]');
      if (new RegExp(site.img, 'i').test(image.src[i])) {
        imgdom.insertAdjacentHTML(
          'afterend',
          `<figcaption class="post-figcaption">&#9650; ${image.title[i]}</figcaption>`
        );
      }
    });

    // 页面加载后为锚链接设置 target
    window.addEventListener('load', function () {
      const linkArr = document.querySelectorAll('.flow a');
      [].forEach.call(linkArr, function (link) {
        if (/^#/i.test(link.href)) {
          link.target = '_self';
        }
      });
    });
  }

  /********
   * archive.html 页面搜索功能
   ********/
  if (page.url === '/archive.html') {
    const archiveInput = document.querySelector('.page-search-input');
    if (archiveInput) {
      archiveInput.addEventListener('keyup', function (e) {
        const archiveLinks = document.getElementsByClassName('archive-item-link');
        const val = this.value.toLowerCase();
        for (let i = 0; i < archiveLinks.length; i++) {
          const link = archiveLinks[i];
          if (link.title.toLowerCase().indexOf(val) > -1) {
            link.closest('li').style.display = 'block';
          } else {
            link.closest('li').style.display = 'none';
          }
        }
        // 回车后跳转搜索页
        if (e.keyCode === 13) {
          location.href = '/search.html?keyword=' + this.value;
        }
      });
    }
  }

  /********
   * search.html 页面搜索功能
   ********/
  if (page.url === '/search.html') {
    const keyword = getQuery('keyword');
    let searchData = [];
    const input = document.querySelector('.search-input');
    const result = document.querySelector('.search-result');

    // 拉取 /search.json
    const xhrSearch = new XMLHttpRequest();
    xhrSearch.open('GET', '/search.json', true);
    xhrSearch.onreadystatechange = function () {
      if (xhrSearch.readyState === 4 && xhrSearch.status === 200) {
        searchData = JSON.parse(xhrSearch.responseText);
        if (keyword) {
          input.value = decodeURI(keyword);
          doSearch(decodeURI(keyword));
        }
        input.placeholder = "请输入关键词，Enter搜索";
      }
    };
    xhrSearch.send(null);

    // 键盘回车执行搜索
    input.addEventListener('keyup', function (e) {
      if (e.keyCode === 13) {
        doSearch(this.value);
      }
    });

    /**
     * [doSearch 执行搜索函数]
     * @param {String} kw - 搜索关键字
     */
    function doSearch(kw) {
      result.innerHTML = '';
      const decWord = decodeURI(kw);
      const title = '搜索：' + decWord + ' | ' + site.title;
      const url = '/search.html?keyword=' + decWord;
      let html = '';

      // 遍历搜索数据
      searchData.forEach(function (item) {
        const postContent = item.title + item.tags.join('') + item.content;
        if (postContent.toLowerCase().indexOf(decWord.toLowerCase()) > -1) {
          const index = item.content.toLowerCase().indexOf(decWord.toLowerCase());
          const realKeyword = item.content.substr(index, decWord.length);
          const first = index > 75 ? index - 75 : 0;
          const last = first + 150;

          html += `
            <div class="search-result-item">
              <i class="search-result-thumb" data-src="${item.thumb}" style="background-image:url(${item.thumb})"></i>
              <div class="search-result-content">
                <div class="search-result-header">
                  <div class="search-result-title">
                    <a class="search-result-link" target="_blank" href="${item.url}">${item.title}</a>
                  </div>
                  <div class="search-result-comment"></div>
                </div>
                <div class="search-result-desc">
                  ${item.content.slice(first, last).replace(new RegExp(realKeyword, 'g'), '<span class="search-result-highlight">' + realKeyword + '</span>')}
                </div>
              </div>
            </div>
          `;
        }
      });
      result.innerHTML = html;

      // 更新标题与浏览器历史
      document.title = title;
      history.replaceState({ "title": title, "url": url }, title, url);
      if (site.home === location.origin && window.parent === window) {
        _hmt.push(['_trackPageview', url]);
      }
    }
  }

  /********
   * tags.html 页面标签搜索功能
   ********/
  if (page.url === '/tags.html') {
    const keyword = getQuery('keyword');
    let tagsData = [];

    // 拉取 /posts.json
    const xhrPosts = new XMLHttpRequest();
    xhrPosts.open('GET', '/posts.json', true);
    xhrPosts.onreadystatechange = function () {
      if (xhrPosts.readyState === 4 && xhrPosts.status === 200) {
        tagsData = JSON.parse(xhrPosts.responseText);
        if (keyword) {
          doTags(decodeURI(keyword));
        }
      }
    };
    xhrPosts.send(null);

    /**
     * [doTags 根据关键字过滤标签并渲染结果]
     * @param {String} kw - 标签关键字
     */
    function doTags(kw) {
      const title = '标签：' + kw + ' | ' + site.title;
      const url = '/tags.html?keyword=' + kw;
      const tagsTable = document.getElementById('tags-table');

      // 显示标签表格
      tagsTable.style.display = 'table';
      tagsTable.querySelector('thead tr').innerHTML = `<th colspan="2">以下是标签含有“${kw}”的所有文章</th>`;

      let html = '';
      tagsData.forEach(function (item) {
        if (item.tags.indexOf(kw) > -1) {
          let date = item.date.slice(0, 10).split('-');
          date = date[0] + ' 年 ' + date[1] + ' 月 ' + date[2] + ' 日';
          html += `
            <tr>
              <td><time>${date}</time></td>
              <td><a href="${item.url}" title="${item.title}">${item.title}</a></td>
            </tr>
          `;
        }
      });
      tagsTable.getElementsByTagName('tbody')[0].innerHTML = html;
      document.title = title;
      history.replaceState({ "title": title, "url": url }, title, url);

      if (site.home === location.origin && window.parent === window) {
        _hmt.push(['_trackPageview', url]);
      }
    }

    // 绑定标签列表点击事件
    const tagLinks = document.getElementsByClassName('post-tags-item');
    const tagCount = tagLinks.length;
    for (let i = 0; i < tagCount; i++) {
      tagLinks[i].addEventListener('click', function (e) {
        e.preventDefault();
        doTags(e.currentTarget.title);
      }, false);
    }
  }

  /********
   * tech.html / life.html / cycling.html - 分页功能
   ********/
  if (page.url === '/life.html' ||
      page.url === '/cycling.html' ||
      page.url === '/tech.html') {

    let pageNum = getQuery('page') ? parseInt(getQuery('page')) : 1;
    let postData = [];
    let posts = [];
    // 根据URL确定分类，例如 /life.html -> 'life'
    const category = page.url.slice(1, -5);  // 'life' / 'cycling' / 'tech'

    // 拉取 /posts.json
    const xhrPosts = new XMLHttpRequest();
    xhrPosts.open('GET', '/posts.json', true);
    xhrPosts.onreadystatechange = function () {
      if (xhrPosts.readyState === 4 && xhrPosts.status === 200) {
        postData = JSON.parse(xhrPosts.responseText);
        // 根据分类筛选文章
        postData.forEach(function (item) {
          if (item.category === category) {
            posts.push(item);
          }
        });
        // 首次渲染
        turn(pageNum, posts, category, disq);
      }
    };
    xhrPosts.send(null);
  }
});

/**
 * [updateZoom 根据窗口高度自适应zoom，解决在特殊屏幕下过大/过小的问题]
 * 仅在高度大于 966 时进行缩放。
 */
function updateZoom() {
  const scale = (window.innerHeight / 966).toFixed(2);
  if (window.innerHeight > 966) {
    document.documentElement.style.setProperty('zoom', parseFloat(scale));
  } else {
    document.documentElement.style.removeProperty('zoom');
  }
}
updateZoom();
window.addEventListener('resize', updateZoom);