document.body.addEventListener('htmx:configRequest', function (evt) {
    const token = getAuthToken();

    if (!token) {
        return;
    }

    evt.detail.headers['Authentication-Token'] = token;
});

function getAuthToken() {
    const token = window.localStorage.getItem('dbmq-auth-token');
    return token;
}

function setToken(e) {
    const token = e.detail.xhr.getResponseHeader('Dbmq-Auth-Token');
    window.localStorage.setItem("dbmq-auth-token", token);
    window.location.reload();
}
