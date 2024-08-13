import './cycling/cycling.scss';

// 为了数据的统一性,generateCalendar处理后赋值供全局使用
let processedActivities = [];

// 日历
function generateCalendar(activities, startDate, numWeeks) {
    const calendarElement = document.getElementById('calendar');
    calendarElement.innerHTML = ''; 
    
    const daysOfWeek = ['一', '二', '三', '四', '五', '六', '日']; 
    daysOfWeek.forEach(day => {
        const dayElement = document.createElement('div');
        dayElement.className = 'calendar-week-header';
        dayElement.innerText = day;
        calendarElement.appendChild(dayElement);
    });

    const todayStr = getChinaTime().toISOString().split('T')[0];
    let currentDate = new Date(startDate);
    currentDate.setUTCHours(0, 0, 0, 0);

    processedActivities = [];
    let todayContainer = null;

    // 创建日历项的函数
    function createDayContainer(date, activities) {
        const dayContainer = document.createElement('div');
        dayContainer.className = 'day-container';

        const dateNumber = document.createElement('span');
        dateNumber.className = 'date-number';
        dateNumber.innerText = date.getUTCDate();
        dayContainer.appendChild(dateNumber);

        const activity = activities.find(activity => activity.activity_time === date.toISOString().split('T')[0]);
        console.log(processedActivities);
        if (activity) processedActivities.push(activity);

        // 根据骑行距离设置球的大小
        const ballSize = activity ? Math.min(parseFloat(activity.riding_distance) / 4, 24) : 2;

        const ball = document.createElement('div');
        ball.className = 'activity-indicator';
        ball.style.width = `${ballSize}px`;
        ball.style.height = `${ballSize}px`;
        if (!activity) ball.classList.add('no-activity');
        ball.style.left = '50%';
        ball.style.top = '50%';
        dayContainer.appendChild(ball);

        dayContainer.addEventListener('mouseenter', () => {
            if (date.toDateString() === new Date().toDateString()) {
                dateNumber.style.opacity = '0';
                ball.style.opacity = '1';
            } else {
                if (todayContainer) {
                    todayContainer.querySelector('.date-number').style.opacity = '0';
                    todayContainer.querySelector('.activity-indicator').style.opacity = '1';
                }
            }
        });
        dayContainer.addEventListener('mouseleave', () => {
            if (date.toDateString() === new Date().toDateString()) {
                dateNumber.style.opacity = '1';
                ball.style.opacity = '0';
            } else {
                if (todayContainer) {
                    todayContainer.querySelector('.date-number').style.opacity = '1';
                    todayContainer.querySelector('.activity-indicator').style.opacity = '0';
                }
            }
        });

        if (date.toDateString() === new Date().toDateString()) {
            // 记录今天的日期容器
            todayContainer = dayContainer;
            dayContainer.classList.add('today');
            ball.style.backgroundColor = '#242428';
            dateNumber.style.color = '#242428';
            dateNumber.style.opacity = '1';
            ball.style.opacity = '0';
        }
        return dayContainer;
    }

    // 异步显示,模仿打字机效果
    async function displayCalendar() {
        for (let week = 0; week < numWeeks; week++) {
            for (let day = 0; day < 7; day++) {
                const currentDateStr = currentDate.toISOString().split('T')[0];
                // 不再计算超过今天的日期
                if (currentDateStr > todayStr) return;

                const dayContainer = createDayContainer(currentDate, activities);
                calendarElement.appendChild(dayContainer);

                // 速度
                await new Promise(resolve => setTimeout(resolve, 30));
                currentDate.setUTCDate(currentDate.getUTCDate() + 1);
            }
        }
    }
    displayCalendar().then(() => {
        generateBarChart();
        displayTotalActivities();
    });
}

// 柱形图
function generateBarChart() {
    const barChartElement = document.getElementById('barChart');
    barChartElement.innerHTML = '';

    const today = getChinaTime();
    const startDate = getStartDate(today, 21);

    // 每周数据
    const weeklyData = {};

    // 每周总活动时间
    processedActivities.forEach(activity => {
        const activityDate = new Date(activity.activity_time);
        const weekStart = getWeekStartDate(activityDate);
        const weekEnd = new Date(weekStart);
        weekEnd.setDate(weekStart.getDate() + 6);

        const weekKey = `${weekStart.toISOString().split('T')[0]} - ${weekEnd.toISOString().split('T')[0]}`;
        weeklyData[weekKey] = (weeklyData[weekKey] || 0) + convertToHours(activity.moving_time);
    });

    // 最大时间
    const maxTime = Math.max(...Object.values(weeklyData), 0);

    // 创建柱形图
    Object.keys(weeklyData).forEach(week => {
        const barContainer = document.createElement('div');
        barContainer.className = 'bar-container';

        const bar = document.createElement('div');
        bar.className = 'bar';
        const width = (weeklyData[week] / maxTime) * 190;
        bar.style.setProperty('--bar-width', `${width}px`);

        const durationText = document.createElement('div');
        durationText.className = 'bar-duration';
        durationText.innerText = '0h';

        const messageBox = createMessageBox();
        const clickMessageBox = createMessageBox();

        barContainer.style.position = 'relative'; 
        bar.appendChild(durationText);
        barContainer.appendChild(bar);
        barContainer.appendChild(messageBox);
        barContainer.appendChild(clickMessageBox);
        barChartElement.appendChild(barContainer);

        bar.style.width = '0';
        bar.offsetHeight;
        // 动画效果
        bar.style.transition = 'width 1s ease-out';
        bar.style.width = `${width}px`;

        durationText.style.opacity = '1';
        // 动态文本
        animateText(durationText, 0, weeklyData[week], 1000);
        setupBarInteractions(bar, messageBox, clickMessageBox, weeklyData[week]);
    });
}

// 动态文本显示
function animateText(element, startValue, endValue, duration) {
    const startTime = performance.now();
    function update() {
        const elapsed = performance.now() - startTime;
        const progress = Math.min(elapsed / duration, 1);
        const currentValue = Math.floor(progress * endValue);
        element.innerText = `${currentValue}h`;
        if (progress < 1) {
            requestAnimationFrame(update);
        } else {
            element.innerText = `${endValue.toFixed(1)}h`;
        }
    }
    update();
}

// 计算总公里数
function calculateTotalKilometers(activities) {
    return activities.reduce((total, activity) => total + parseFloat(activity.riding_distance) || 0, 0);
}

// 显示总活动数和总公里数
function displayTotalActivities() {
    const totalCountElement = document.getElementById('totalCount');
    const totalDistanceElement = document.getElementById('totalDistance');

    if (!totalCountElement || !totalDistanceElement) return;

    const totalCountValue = totalCountElement.querySelector('#totalCountValue');
    const totalDistanceValue = totalDistanceElement.querySelector('#totalDistanceValue');

    const totalCountSpinner = totalCountElement.querySelector('.loading-spinner');
    const totalDistanceSpinner = totalDistanceElement.querySelector('.loading-spinner');

    totalCountSpinner.classList.add('active');
    totalDistanceSpinner.classList.add('active');

    const uniqueDays = new Set(processedActivities.map(activity => activity.activity_time));
    const totalCount = uniqueDays.size;
    const totalKilometers = calculateTotalKilometers(processedActivities);

    animateCount(totalCountValue, totalCount, 1000, 50);
    animateCount(totalDistanceValue, totalKilometers, 1000, 50, true);

    setTimeout(() => {
        totalDistanceValue.textContent = `${totalKilometers.toFixed(2)} km`;
        totalCountSpinner.classList.remove('active');
        totalDistanceSpinner.classList.remove('active');
    }, 1000);
}

// 获取一周的开始日期
function getWeekStartDate(date) {
    const day = date.getDay();
    const diff = (day === 0 ? -6 : 1) - day;
    const weekStart = new Date(date);
    weekStart.setDate(weekStart.getDate() + diff);
    return weekStart;
}

// 将JSON的时间数据转换为小时
function convertToHours(moving_time) {
    const [hours, minutes] = moving_time.split(':').map(Number);
    return hours + (minutes / 60);
}

// 博客托管Github Pages需要中国时间
function getChinaTime() {
    const now = new Date();
    const offset = 8 * 60 * 60 * 1000;
    return new Date(now.getTime() + offset);
}

// 手搓JSON
async function loadActivityData() {
    const response = await fetch('https://cos.lhasa.icu/data/cycling_data.json');
    return response.json();
}

(async function() {
    const today = getChinaTime();
    const startDate = getStartDate(today, 21);

    const activities = await loadActivityData();
    generateCalendar(activities, startDate, 4);
})();

// 创建消息盒子
function createMessageBox() {
    const messageBox = document.createElement('div');
    messageBox.className = 'message-box';
    return messageBox;
}

// 获取起始时间
function getStartDate(today, daysOffset) {
    const currentDayOfWeek = today.getUTCDay();
    const daysToMonday = (currentDayOfWeek === 0 ? 6 : currentDayOfWeek - 1);
    const startDate = new Date(today);
    startDate.setUTCDate(today.getUTCDate() - daysToMonday - daysOffset);
    return startDate;
}

// 动态更新计数器
function animateCount(element, totalValue, duration, intervalDuration, isDistance = false) {
    const step = totalValue / (duration / intervalDuration);
    let count = 0;
    const interval = setInterval(() => {
        count += step;
        if (count >= totalValue) {
            count = totalValue;
            clearInterval(interval);
        }
        element.textContent = isDistance ? count.toFixed(2) : Math.round(count);
    }, intervalDuration);
}

// 骚话集合
function setupBarInteractions(bar, messageBox, clickMessageBox, weeklyData) {
    let mouseLeaveTimeout;
    let autoHideTimeout;

    bar.addEventListener('mouseenter', () => {
        clearTimeout(mouseLeaveTimeout);
        clearTimeout(autoHideTimeout);

        const message = weeklyData > 14 ? '这周干的还不错' : '偷懒了啊';
        messageBox.innerText = message;
        messageBox.classList.add('show');

        autoHideTimeout = setTimeout(() => {
            messageBox.classList.remove('show');
        }, 700);
    });

    bar.addEventListener('mouseleave', () => {
        mouseLeaveTimeout = setTimeout(() => {
            messageBox.classList.remove('show');
        }, 700);
    });

    bar.addEventListener('click', () => {
        clickMessageBox.innerText = '一起来运动吧！';
        clickMessageBox.classList.add('show');
        setTimeout(() => {
            clickMessageBox.classList.remove('show');
        }, 700);

        messageBox.classList.remove('show');
        clearTimeout(mouseLeaveTimeout);
        clearTimeout(autoHideTimeout);
    });
}