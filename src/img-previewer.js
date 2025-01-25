import ImgPreviewer from'img-previewer'
import './img-previewer/_img-previewer.scss'

document.addEventListener('DOMContentLoaded', function () {
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
  }
})