let paymentOrderIds = [];
let paymentFromCart = false;

document.addEventListener("DOMContentLoaded", async () => {
    const params = new URLSearchParams(window.location.search);
    const raw = params.get("orders") || "";
    paymentFromCart = params.get("cart") === "1";
    const ids = raw.split(",");
    const payBtn = document.getElementById("pay-btn");
    payBtn.disabled = true;
    if (!raw || ids.some(id => !/^[1-9]\d*$/.test(id))) {
        document.getElementById("amount-display").innerText = "注文IDが不正です。";
        return;
    }
    paymentOrderIds = ids.map(Number);
    if (paymentOrderIds.some(id => !Number.isSafeInteger(id)) || new Set(paymentOrderIds).size !== paymentOrderIds.length) {
        document.getElementById("amount-display").innerText = "注文IDが不正です。";
        return;
    }
    try {
        const response = await fetch("/api/orders");
        if (!response.ok) throw new Error("注文情報を取得できません。");
        const orders = await response.json();
        let total = 0;
        for (const id of paymentOrderIds) {
            const order = orders.find(o => o.id === id);
            if (!order) throw new Error("対象の注文が見つかりません。");
            total += order.price * order.quantity;
        }
        if (!Number.isSafeInteger(total) || total <= 0) throw new Error("金額が不正です。");
        document.getElementById("amount-display").innerText = `支払金額：¥${total.toLocaleString()}`;
        payBtn.disabled = false;
    } catch (error) {
        document.getElementById("amount-display").innerText = error.message;
    }
    document.getElementById("payment-form").addEventListener("submit", executePayment);
});

async function executePayment(event) {
    event.preventDefault();
    const payBtn = document.getElementById("pay-btn");
    const method = document.querySelector('input[name="method"]:checked');
    if (!method) { alert("支払方法を選択してください。"); return; }
    payBtn.disabled = true;
    try {
        const response = await fetch("/api/payments", {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify({order_ids: paymentOrderIds, method: method.value})
        });
        if (!response.ok) throw new Error(response.status === 409 ? "既に決済済みの注文が含まれています。" : "決済処理に失敗しました。");
        if (paymentFromCart) localStorage.removeItem("cart");
        document.getElementById("message").innerText = "決済が完了しました";
        document.getElementById("message").style.display = "block";
        window.location.href = "orders.html";
    } catch (error) {
        alert(error.message);
        payBtn.disabled = false;
    }
}
