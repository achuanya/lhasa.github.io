var iDisqus = require('disqus-php-api');
import './sass/main.scss';

// Img-previewer https://github.com/yue1123/img-previewer
import ImgPreviewer from'img-previewer'
import 'img-previewer/dist/index.css'
let timer;

// 百度统计
var _hmt = _hmt || [];

// TimeAgo https://coderwall.com/p/uub3pw/javascript-timeago-func-e-g-8-hours-ago
// 时间格式化函数
function timeAgo(selector) {
  var templates = {
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

  var template = function (t, n) {
    return templates[t] && templates[t].replace(/%d/i, Math.abs(Math.round(n)));
  };

  var timer = function (time) {
    if (!time) return;
    // 移除毫秒
    time = time.replace(/\.\d+/, "");
    time = time.replace(/-/, "/").replace(/-/, "/");
    time = time.replace(/T/, " ").replace(/Z/, " UTC");
    // -04:00 -> -0400
    time = time.replace(/([\+\-]\d\d)\:?(\d\d)/, " $1$2");
    time = new Date(time * 1000 || time);

    var now = new Date();
    var seconds = ((now.getTime() - time) * .001) >> 0;
    var minutes = seconds / 60;
    var hours = minutes / 60;
    var days = hours / 24;
    var years = days / 365;

    return templates.prefix + (
      seconds < 45 && template('seconds', seconds) || seconds < 90 && template('minute', 1) || minutes < 45 && template('minutes', minutes) || minutes < 90 && template('hour', 1) || hours < 24 && template('hours', hours) || hours < 42 && template('day', 1) || days < 30 && template('days', days) || days < 45 && template('month', 1) || days < 365 && template('months', days / 30) || years < 1.5 && template('year', 1) || template('years', years)) + templates.suffix;
  };

  var elements = document.getElementsByClassName('timeago');
  for (var i in elements) {
    var $this = elements[i];
    if (typeof $this === 'object') {
      $this.innerHTML = timer($this.getAttribute('datetime'));
    }
  }
  // 每分钟更新一次时间
  setTimeout(timeAgo, 60000);
}

// 获取URL查询参数
function getQuery(variable) {
  var query = window.location.search.substring(1);
  var vars = query.split("&");
  for (var i = 0; i < vars.length; i++) {
    var pair = vars[i].split("=");
    if (pair[0] == variable) { return pair[1]; }
  }
  return (false);
}

// 页面关闭时取消菜单选中状态
window.addEventListener('beforeunload', function (event) {
  document.getElementById('menu').checked = false;
});

document.addEventListener('DOMContentLoaded', function (event) {

  // Img-previewer
  if (document.querySelector('.post-content')) {
    const a = new ImgPreviewer('.post-content', {
      scrollbar: true,
      ratio: 0.7,
      imageZoom: {
        step: 1
      },
      style: {
        modalOpacity: 0.8
      },
      bubblingLevel: 1,
      onHide() {
        clearInterval(timer);
      }
    });
  } else {
    console.log('No .post-content element found on this page.');
  }

  // Disqus
  var disq = new iDisqus('comment', {
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
  timeAgo();

  // 检查是否为文章页面
  if (page.layout == 'post') {
    var imageArr = document.querySelectorAll('.post-content img[data-src]:not([class="emoji"])')
    // console.log(imageArr);
    var image = {
      src: [],
      url: [],
      thumb: [],
      title: [],
      coord: []
    };
    // 收集图片的相关信息
    for (var i = 0; i < imageArr.length; i++) {
      image.thumb[i] = imageArr[i].src;
      image.src[i] = imageArr[i].dataset.src;
      image.url[i] = imageArr[i].dataset.url;
      //new RegExp(site.img,'i').test(imageArr[i].src) ? imageArr[i].src.split(/_|\?/)[0] : imageArr[i].src;
    }
    // 过滤出 jpg 图片
    image.jpg = image.src.filter(function (item) {
      return item.indexOf('.jpg') > -1 && new RegExp(site.img, 'i').test(item);
    });
    // 为每张图片添加标题和相关事件
    [].forEach.call(imageArr, function (item, i) {
      image.title[i] = item.title || item.parentElement.textContent.trim() || item.alt;
      item.title = image.title[i];
      item.classList.add('post-image');
      item.setAttribute('data-index', i);
      item.parentElement.outerHTML = item.parentElement.outerHTML
      .replace('<p>', '<figure class="post-figure" data-index=' + i + '>')
      .replace('<p>', '<figure class="post-figure" data-index=' + i + '>')
      .replace('</p>', '</figure>')
      .replace(item.parentElement.textContent, '');
      
      var imgdom = document.querySelector('.post-image[data-src="' + image.src[i] + '"]');
      if (new RegExp(site.img, 'i').test(image.src[i])) {
        imgdom.insertAdjacentHTML('afterend', '<figcaption class="post-figcaption">&#9650; ' + image.title[i] + '</figcaption>');
      }

      // imgdom.addEventListener('click', function () {

      // });

    })

    // 获取图片的 EXIF 信息
    var getExif = function (index) {
      if (index < image.jpg.length) {
        var item = image.jpg[index];
        var xhrExif = new XMLHttpRequest();
        xhrExif.open('GET', item + '?exif', true);
        xhrExif.onreadystatechange = function () {
          if (this.readyState == 4) {
            if (this.status == 200) {
              var data = JSON.parse(this.responseText);
              var parseVal = function (odata) {
                if (!!odata) {
                  return odata.val;
                } else {
                  return '无';
                }
              }
              if (!!data.DateTimeOriginal) {
                var datetime = data.DateTimeOriginal.val.split(/\:|\s/);
                var date = datetime[0] + '-' + datetime[1] + '-' + datetime[2] + ' ' + datetime[3] + ':' + datetime[4];
                var make = parseVal(data.Make);
                var model = parseVal(data.Model);
                var fnum = parseVal(data.FNumber);
                var extime = parseVal(data.ExposureTime);
                var iso = parseVal(data.ISOSpeedRatings);
                var flength = parseVal(data.FocalLength);
                document.querySelector('.post-image[data-src="' + item + '"]').closest('.post-figure').dataset.exif = '时间: ' + date + ' 器材: ' + (model.indexOf(make) > -1 ? '' : make) + ' ' + model + ' 光圈: ' + fnum + ' 快门: ' + extime + ' 感光度: ' + iso + ' 焦距: ' + flength;
              }
              if (!!data.GPSLongitude) {
                var olat = data.GPSLatitude.val.split(', ');
                var olng = data.GPSLongitude.val.split(', ');
                var lat = 0, lng = 0;
                for (var e = 0; e < olat.length; e++) {
                  lat += olat[e] / Math.pow(60, e);
                  lng += olng[e] / Math.pow(60, e);
                }
                lat = data.GPSLatitudeRef && data.GPSLatitudeRef.val == 'S' ? -lat : lat;
                lng = data.GPSLongitudeRef && data.GPSLongitudeRef.val == 'W' ? -lng : lng;
                image.coord[index] = coordtransform.wgs84togcj02(lng, lat).join(',');
              }
            }
            index++;
            getExif(index);
          }
        }
        xhrExif.send();
      } else {
        var xhrRegeo = new XMLHttpRequest();
        xhrRegeo.open('GET', site.api + '/regeo.php?coords=' + image.coord.filter(function () { return true }).join('|'), true);
        xhrRegeo.onreadystatechange = function () {
          if (this.readyState == 4 && this.status == 200) {
            var data = JSON.parse(this.responseText);
            if (data.info == 'OK') {
              var address, city, dist, town;
              for (var m = 0, n = 0; m < image.jpg.length; m++) {
                address = data.regeocodes[n];
                if (m in image.coord && !!address) {
                  address = address.addressComponent;
                  city = address.city || '';
                  dist = address.district || '';
                  town = address.township || '';
                  document.querySelector('[data-src="' + image.jpg[m] + '"]').title = '摄于' + city + dist + town;
                  n++;
                }
              }
            }
          }
        }
        xhrRegeo.send();
      }
    }

    // 如果存在 jpg 图片，则获取其 EXIF 信息
    if (image.jpg.length > 0) {
      getExif(0);
    }

    // 页面加载后为锚链接设置目标属性
    window.addEventListener('load', function () {
      var linkArr = document.querySelectorAll('.flow a');
      [].forEach.call(linkArr, function (link) {
        if (/^#/i.test(link.href)) {
          link.target = '_self';
        }
      })
    });

    // 获取相关文章数据并显示随机相关文章
    var postData;
    var xhrPosts = new XMLHttpRequest();
    xhrPosts.open('GET', '/posts.json', true);
    xhrPosts.onreadystatechange = function () {
      if (xhrPosts.readyState == 4 && xhrPosts.status == 200) {
        postData = JSON.parse(xhrPosts.responseText);
        randomPosts(relatedPosts(page.tags, page.category));
      }
    }
    xhrPosts.send(null);

    function relatedPosts(tags, cat) {
      var posts = [];
      var used = [];
      postData.forEach(function (item, i) {
        if (item.tags.some(function (tag) { return tags.indexOf(tag) > -1; }) && item.url != location.pathname) {
          posts.push(item);
          used.push(i);
        }
      })
      while (posts.length < 5) {
        var index = Math.floor(Math.random() * postData.length);
        var item = postData[index];
        if (used.indexOf(index) == '-1' && item.category == cat && item.url != location.pathname) {
          posts.push(item);
          used.push(index);
        }
      }
      return posts;
    }

    function randomPosts(posts) {
      var used = [];
      var counter = 0;
      var html = '';
      while (counter < 5) {
        var index = Math.floor(Math.random() * posts.length);
        if (used.indexOf(index) == '-1') {
          html += '<li class="post-extend-item"><a class="post-extend-link" href="' + posts[index].url + '" title="' + posts[index].title + '">' + posts[index].title + '</a></li>\n';
          used.push(index);
          counter++;
        }
      }
      document.querySelector('#random-posts').insertAdjacentHTML('beforeend', html);
    }
  }

  // 处理 archive.html 页面搜索功能
  if (page.url == '/archive.html') {
    document.querySelector('.page-search-input').addEventListener('keyup', function (e) {
      var archive = document.getElementsByClassName('archive-item-link');
      for (var i = 0; i < archive.length; i++) {
        if (archive[i].title.toLowerCase().indexOf(this.value.toLowerCase()) > -1) {
          archive[i].closest('li').style.display = 'block';
        } else {
          archive[i].closest('li').style.display = 'none';
        }
      }
      if (e.keyCode == 13) {
        location.href = '/search.html?keyword=' + this.value;
      }
    })
  }

  // 处理 search.html 页面搜索功能
  if (page.url == '/search.html') {
    var keyword = getQuery('keyword');
    var searchData;
    var input = document.querySelector('.search-input');
    var result = document.querySelector('.search-result');
    var xhrSearch = new XMLHttpRequest();
    xhrSearch.open('GET', '/search.json', true);
    xhrSearch.onreadystatechange = function () {
      if (xhrSearch.readyState == 4 && xhrSearch.status == 200) {
        searchData = JSON.parse(xhrSearch.responseText);
        if (keyword) {
          input.value = decodeURI(keyword);
          search(decodeURI(keyword));
        }
        input.placeholder = "请输入关键词，Enter搜索";
      }
    }
    xhrSearch.send(null);

    document.querySelector('.search-input').addEventListener('keyup', function (e) {
      if (e.keyCode == 13) {
        search(decodeURI(this.value));
      }
    })

    function search(keyword) {
      // 清空搜索结果区域
      result.innerHTML = '';
      // 设置页面标题
      var title = '搜索：' + keyword + ' | ' + site.title;
      // 构建搜索结果的URL
      var url = '/search.html?keyword=' + keyword;
      // 获取搜索结果的总数
      var total = result.length;
      // 初始化HTML字符串
      var html = '';

      // 遍历搜索数据
      searchData.forEach(function (item) {
        // 拼接内容
        var postContent = item.title + item.tags.join('') + item.content;
        // 检查关键字是否出现在内容中（不区分大小写）
        if (postContent.toLowerCase().indexOf(keyword.toLowerCase()) > -1) {
          // 获取关键字在内容中的索引
          var index = item.content.toLowerCase().indexOf(keyword.toLowerCase());
          // 获取实际关键字
          var realKeyword = item.content.substr(index, keyword.length);
          // 确定内容片段的起始和结束位置
          var first = index > 75 ? index - 75 : 0;
          var last = first + 150;
          // 构建搜索结果的HTML
          html += '<div class="search-result-item">' +
            '      <i class="search-result-thumb" data-src="' + item.thumb + '" style="background-image:url(' + item.thumb + ')"></i>' +
            '      <div class="search-result-content">' +
            '        <div class="search-result-header">' +
            '           <div class="search-result-title"><a class="search-result-link" target="_blank" href="' + item.url + '">' + item.title + '</a></div>' +
            '           <div class="search-result-comment"></div>' +
            '        </div>' +
            '        <div class="search-result-desc">' + item.content.slice(first, last).replace(new RegExp(realKeyword, 'g'), '<span class="search-result-highlight">' + realKeyword + '</span>') + '</div>' +
            '      </div>' +
            '    </div>';
        }
      })
      // 更新搜索结果区域的HTML内容
      result.innerHTML = html;
      // 更新页面标题
      document.title = title;
      // 使用History API更新浏览器历史记录
      history.replaceState({
        "title": title,
        "url": url
      }, title, url);

      // 如果当前页面是主页且没有嵌入在iframe中，推送页面视图到统计工具
      if (site.home === location.origin && window.parent == window) {
        _hmt.push(['_trackPageview', url]);
      }
    }
  }

  // 处理 tags.html 页面标签搜索功能
  if (page.url == '/tags.html') {
    var keyword = getQuery('keyword');
    var tagsData;
    var xhrPosts = new XMLHttpRequest();
    xhrPosts.open('GET', '/posts.json', true);
    xhrPosts.onreadystatechange = function () {
      if (xhrPosts.readyState == 4 && xhrPosts.status == 200) {
        tagsData = JSON.parse(xhrPosts.responseText);
        if (keyword) {
          tags(decodeURI(keyword));
        }
      }
    }
    xhrPosts.send(null);
    function tags(keyword) {
      var title = '标签：' + keyword + ' | ' + site.title;
      var url = '/tags.html?keyword=' + keyword;
      var tagsTable = document.getElementById('tags-table');
      tagsTable.style.display = 'table';
      tagsTable.querySelector('thead tr').innerHTML = '<th colspan=2>以下是标签含有“' + keyword + '”的所有文章</th>';
      var html = '';
      tagsData.forEach(function (item) {
        if (item.tags.indexOf(keyword) > -1) {
          var date = item.date.slice(0, 10).split('-');
          date = date[0] + ' 年 ' + date[1] + ' 月 ' + date[2] + ' 日';
          html += '<tr><td><time>' + date + '</time></td><td><a href="' + item.url + '" title="' + item.title + '">' + item.title + '</a></td></tr>';
        }
      })
      tagsTable.getElementsByTagName('tbody')[0].innerHTML = html;
      document.title = title;
      history.replaceState({
        "title": title,
        "url": url
      }, title, url);
      if (site.home === location.origin && window.parent == window) {
        _hmt.push(['_trackPageview', url]);
      }
    }
    var tagLinks = document.getElementsByClassName('post-tags-item');
    var tagCount = tagLinks.length;
    for (var i = 0; i < tagCount; i++) {
      tagLinks[i].addEventListener('click', function (e) {
        tags(e.currentTarget.title);
        e.preventDefault();
      }, false);
    }
  }

  // 处理 tech.html, life.html 页面分页功能
  if (page.url == '/tech.html' || page.url == '/life.html') {
    // 获取当前页面的页码，如果没有则默认为第一页
    var pageNum = !!getQuery('page') ? parseInt(getQuery('page')) : 1;
    var postData, posts = [];
    var xhrPosts = new XMLHttpRequest();
    // 根据页面的 URL 来确定分类
    var category = page.url.slice(1, -5);
    xhrPosts.open('GET', '/posts.json', true);
    xhrPosts.onreadystatechange = function () {
      // 当数据加载完成且成功时
      if (xhrPosts.readyState == 4 && xhrPosts.status == 200) {
        // 解析获取到的文章数据
        postData = JSON.parse(xhrPosts.responseText);
        // 根据分类筛选出符合条件的文章
        postData.forEach(function (item) {
          if (item.category == category) {
            posts.push(item);
          }
        })
        // 根据当前页码显示对应页的文章列表
        turn(pageNum);
      }
    }
    xhrPosts.send(null);

    // 定义函数 turn，用于生成对应页的文章列表和分页控件
    function turn(pageNum) {
      var cat = '';
      var postClass = '';
      var pageSize = 10;
      // 根据不同的页面 URL 设置分类名称和文章列表样式类
      switch (page.url) {
        case '/tech.html':
          cat = '技术';
          postClass = 'post-tech';
          break;
        case '/life.html':
          cat = '生活';
          // 生活页面每页显示 12 篇文章
          pageSize = 12;
          postClass = 'post-life';
          break;
      }
      // 根据当前页码计算文章起始和结束位置
      var title = pageNum == 1 ? cat + ' | ' + site.title : cat + '：第' + pageNum + '页 | ' + site.title;
      var url = pageNum == 1 ? page.url : page.url + '?page=' + pageNum;
      var html = '';
      var total = posts.length;
      var first = (pageNum - 1) * pageSize;
      var last = total > pageNum * pageSize ? pageNum * pageSize : total;
      // 根据不同页面的文章列表格式生成 HTML
      if (page.url == '/life.html') {
        // 生活页面特定格式
        for (var i = first; i < last; i++) {
          var item = posts[i];
          html += '<article class="post-item">' +
            '    <i class="post-item-thumb" data-src="' + item.image + '" style="background-image:url(' + (item.image.indexOf('svg') > -1 ? item.image : item.image + '?imageView2/1/w/400/h/266') + ')"></i>' +
            '    <section class="post-item-summary">' +
            '    <h3 class="post-item-title"><a class="post-item-link" href="' + item.url + '" title="' + item.title + '">' + item.title + (item.images > 30 && item.category == 'life' ? '[' + item.images + 'P]' : '') + '</a></h3>' +
            '    </section>' +
            '    <section class="post-item-footer"><time class="post-item-date timeago" datetime="' + item.date + '"></time><a class="post-item-cmt" title="查看评论" href="' + item.url + '#comment"><span data-disqus-url="' + item.url + '"></span><span>条评论</span></a></section>' +
            '</article>';
        }
      } else {
        // 其他页面的通用格式
        for (var i = first; i < last; i++) {
          var item = posts[i];
          html += '<article class="post-item">' +
            '    <i class="post-item-thumb" data-src="' + item.thumb + '" style="background-image:url(' + item.thumb + ')"></i>' +
            '    <section class="post-item-summary">' +
            '    <h3 class="post-item-title"><a class="post-item-link" href="' + item.url + '" title="' + item.title + '">' + item.title + (item.images > 30 && item.category == 'life' ? '[' + item.images + 'P]' : '') + '</a></h3>' +
            '    <time class="post-item-date timeago" datetime="' + item.date + '"></time>' +
            '    </section>' +
            '    <a class="post-item-comment" title="查看评论" data-disqus-url="' + item.url + '" href="' + item.url + '#comment"></a>' +
            '</article>';
        }
      }

      // 计算总页数，并生成分页控件的 HTML
      var totalPage = Math.ceil(total / pageSize);
      var prev = pageNum > 1 ? pageNum - 1 : 0;
      var next = pageNum < totalPage ? pageNum + 1 : 0;
      var prevLink = !!prev ? '<a class="pagination-item-link" href="' + page.url + '?page=' + prev + '" data-page="' + prev + '">较新文章 &raquo;</a>' : '';
      var nextLink = !!next ? '<a class="pagination-item-link" href="' + page.url + '?page=' + next + '" data-page="' + next + '">&laquo; 较旧文章</a>' : '';
      var pagination = '<ul class="pagination-list">' +
        '<li class="pagination-item">' + nextLink + '</li>' +
        '<li class="pagination-item">' + pageNum + ' / ' + totalPage + '</li>' +
        '<li class="pagination-item">' + prevLink + '</li>' +
        '</ul>';

      // 将生成的文章列表和分页控件插入到页面中
      // 添加文章列表的样式类
      document.querySelector('.post-list').classList.add(postClass);
      // 插入文章列表的 HTML 内容
      document.querySelector('.post-list').innerHTML = html;
      // 插入分页控件的 HTML 内容
      document.querySelector('.pagination').innerHTML = pagination;
      // 更新文章列表中的时间显示
      timeAgo();
      // 更新文章列表中评论数量的显示
      disq.count();
      // 给分页链接添加点击事件，点击时切换到对应页码的文章列表
      var link = document.getElementsByClassName('pagination-item-link');
      for (var i = 0; i < link.length; i++) {
        link[i].addEventListener('click', function (e) {
          var pageNum = parseInt(e.currentTarget.dataset.page);
          turn(pageNum);
          e.preventDefault();
        })
      }
      // 更新页面标题和浏览器历史记录，用于页面切换时的状态保持
      document.title = title;
      history.replaceState({
        "title": title,
        "url": url
      }, title, url);
      if (site.home === location.origin && window.parent == window) {
        _hmt.push(['_trackPageview', url]);
      }
    }
  }
})