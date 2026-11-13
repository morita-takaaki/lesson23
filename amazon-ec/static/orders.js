document.addEventListener("DOMContentLoaded", async () => {
    const tbody = document.getElementById("order-items");
    try {
        const me = await fetch("/api/me");
        if (!me.ok) {
            tbody.innerHTML = '<tr><td colspan="5">自分の注文履歴を見るにはログインしてください。<a href="login.html">ログイン</a></td></tr>';
            return;
        }
        const response = await fetch("/api/orders");
        if (!response.ok) throw new Error("注文履歴の取得に失敗しました。");
        const orders = await response.json();
        tbody.textContent = "";
        if (!orders.length) {
            tbody.innerHTML = '<tr><td colspan="5">自分の注文履歴はありません。</td></tr>';
            return;
        }
        for (const item of orders) {
            const row = document.createElement("tr");
            const values = [item.product_name, `¥${item.price.toLocaleString()}`, String(item.quantity), `¥${(item.price * item.quantity).toLocaleString()}`, item.shipping_status];
            for (const value of values) {
                const cell = document.createElement("td");
                cell.textContent = value;
                row.appendChild(cell);
            }
            tbody.appendChild(row);
        }
    } catch (error) {
        console.error(error);
        tbody.innerHTML = '<tr><td colspan="5">注文履歴の取得に失敗しました。</td></tr>';
    }
});
