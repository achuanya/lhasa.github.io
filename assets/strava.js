const clientId = '130832';
const clientSecret = 'cd2f3b989e5f1edea85e758a87434e47e08d97af';
let accessToken = '89050cc9dc217ab1fe80558dbd2523687118d1ab';
const refreshToken = '31ae71ec99deb6be48629ce52af2915b01eba41f';

// 更新 Access Token 的函数
function refreshAccessToken() {
    return $.ajax({
        url: 'https://www.strava.com/oauth/token',
        method: 'POST',
        data: {
            client_id: clientId,
            client_secret: clientSecret,
            refresh_token: refreshToken,
            grant_type: 'refresh_token'
        },
        success: function(response) {
            accessToken = response.access_token;
            console.log('Access token refreshed');
            getRecentRides(); // 获取新的访问令牌后再次调用
        },
        error: function(jqXHR) {
            console.log('Error refreshing access token:', jqXHR);
        }
    });
}

// 获取最近的骑行记录
function getRecentRides() {
    $.ajax({
        url: 'https://www.strava.com/api/v3/athlete/activities',
        headers: {
            'Authorization': 'Bearer ' + accessToken
        },
        data: {
            per_page: 5
        },
        success: function(data) {
            displayRides(data);
        },
        error: function(jqXHR) {
            console.log('Error fetching rides:', jqXHR.status, jqXHR.responseText);
            if (jqXHR.status === 401) { // Unauthorized
                refreshAccessToken();
            }
        }
    });
}

// 显示骑行记录
function displayRides(rides) {
    const ridesContainer = $('#rides');
    ridesContainer.empty();
    rides.forEach(ride => {
        const rideElement = $('<div></div>').text(`骑行时间: ${ride.start_date}, 距离: ${ride.distance} 米`);
        ridesContainer.append(rideElement);
    });
}

// 初始化函数
function init() {
    getRecentRides();
}

$(document).ready(init);