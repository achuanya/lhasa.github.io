function generateCalendar(startDate, numWeeks, activities) {
    const calendarElement = document.getElementById('calendar');
    calendarElement.innerHTML = ''; // 清空现有内容

    const daysOfWeek = ['一', '二', '三', '四', '五', '六', '日']; // 星期标题按照顺序排列
    daysOfWeek.forEach(day => {
        const dayElement = document.createElement('div');
        dayElement.className = 'header';
        dayElement.innerText = day;
        calendarElement.appendChild(dayElement);
    });

    // 生成日历
    let currentDate = new Date(startDate);

    for (let week = 0; week < numWeeks; week++) {
        for (let day = 0; day < 7; day++) {
            const dayContainer = document.createElement('div');
            dayContainer.className = 'day-container';

            const dateNumber = document.createElement('span');
            dateNumber.className = 'date-number';
            dateNumber.innerText = currentDate.getDate();
            dayContainer.appendChild(dateNumber);

            // 查找活动数据
            const activity = activities.find(activity => activity.activity_time === currentDate.toISOString().split('T')[0]);
            const ballSize = activity ? Math.min(parseFloat(activity.riding_distance) / 5, 30) : 10; // 最大球的尺寸为 30px

            const ball = document.createElement('div');
            ball.className = 'activity-indicator';
            ball.style.width = `${ballSize}px`;
            ball.style.height = `${ballSize}px`;
            ball.style.left = '50%'; // 居中对齐
            ball.style.top = '50%'; // 居中对齐
            dayContainer.appendChild(ball);

            // 鼠标悬停效果
            dayContainer.addEventListener('mouseenter', () => {
                dateNumber.style.opacity = '1'; // 显示日期数字
                ball.style.opacity = '0'; // 隐藏球
            });
            dayContainer.addEventListener('mouseleave', () => {
                dateNumber.style.opacity = '0'; // 隐藏日期数字
                ball.style.opacity = '1'; // 显示球
            });

            // 高亮今天的日期
            if (currentDate.toDateString() === new Date().toDateString()) {
                dayContainer.classList.add('today');
            }

            calendarElement.appendChild(dayContainer);
            currentDate.setDate(currentDate.getDate() + 1);
        }
    }
}

// 获取当前中国时间
function getChinaTime() {
    const now = new Date();
    const offset = 8 * 60 * 60 * 1000; // 中国时区偏移量（+8小时）
    return new Date(now.getTime() + offset);
}

// 读取 JSON 数据
async function loadActivityData() {
    const response = await fetch('https://cos.lhasa.icu/data/cycling_data.json');
    const data = await response.json();
    return data;
}

// 生成过去四周的日历
(async function() {
    const today = getChinaTime();
    const currentDayOfWeek = today.getDay();
    const daysToMonday = (currentDayOfWeek === 0 ? 6 : currentDayOfWeek - 1); // 计算到周一的天数
    const startDate = new Date(today);
    startDate.setDate(today.getDate() - daysToMonday - 21); // 计算三周前的周一
    startDate.setDate(startDate.getDate() - (startDate.getDay() === 0 ? 6 : startDate.getDay() - 1)); // 让开始日期对齐到周一

    const activities = await loadActivityData();
    generateCalendar(startDate, 4, activities);
})();
