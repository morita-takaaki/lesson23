async function updateUser() {
    const response = await fetch("/api/me");
    const user = response.ok ? await response.json() : null;
    document.getElementById("current-user").textContent = user ? `${user.username} さんがログイン中です` : "現在ログインしていません";
}
document.getElementById("login-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const response = await fetch("/api/login", {method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify({username:document.getElementById("username").value, password:document.getElementById("password").value})});
    document.getElementById("message").textContent = response.ok ? "ログインしました。" : "ログインできませんでした。";
    if (response.ok) { document.getElementById("password").value = ""; await updateUser(); }
});
document.getElementById("logout").addEventListener("click", async () => {
    const response = await fetch("/api/logout", {method:"POST"});
    document.getElementById("message").textContent = response.ok ? "ログアウトしました。" : "ログアウトできませんでした。";
    await updateUser();
});
updateUser();
