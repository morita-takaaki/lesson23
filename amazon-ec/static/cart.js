document.addEventListener("DOMContentLoaded", () => {
    displayCart();
    const checkoutBtn = document.getElementById("checkout-btn");
    if (checkoutBtn) {
        checkoutBtn.onclick = () => checkoutCart();
    }
});

function displayCart() {
    const cart = JSON.parse(localStorage.getItem("cart")) || [];
    const cartItemsTbody = document.getElementById("cart-items");
    const cartTotalHeader = document.getElementById("cart-total");
    cartItemsTbody.innerHTML = "";

    if (cart.length === 0) {
        cartItemsTbody.innerHTML = `
            <tr>
                <td colspan="4" style="text-align: center;">カートに商品が入っていません。</td>
            </tr>
        `;
        cartTotalHeader.innerText = "合計: ¥0";
        return;
    }

    let total = 0;
    cart.forEach(item => {
        const subtotal = item.price * item.quantity;
        total += subtotal;
        const row = document.createElement("tr");
        row.innerHTML = `
            <td>${escapeHtml(item.name)}</td>
            <td style="text-align: right;">¥${item.price.toLocaleString()}</td>
            <td style="text-align: center;">${item.quantity}</td>
            <td style="text-align: right;">¥${subtotal.toLocaleString()}</td>
        `;
        cartItemsTbody.appendChild(row);
    });

    cartTotalHeader.innerText = `合計: ¥${total.toLocaleString()}`;
}

async function checkoutCart() {
    const cart = JSON.parse(localStorage.getItem("cart")) || [];
    if (cart.length === 0) {
        alert("カートが空です。");
        return;
    }

    const checkoutBtn = document.getElementById("checkout-btn");
    checkoutBtn.disabled = true;
    const orderIds = [];
    try {
        for (const item of cart) {
            const response = await fetch("/api/orders", {
                method: "POST",
                headers: {"Content-Type": "application/json"},
                body: JSON.stringify({product_id: item.id, quantity: item.quantity})
            });
            if (!response.ok) throw new Error("注文登録に失敗しました。");
            const data = await response.json();
            if (!Number.isSafeInteger(data.id) || data.id <= 0) throw new Error("注文ID取得失敗");
            orderIds.push(data.id);
        }
        // 決済完了までカートは消さない
        window.location.href = `payment.html?orders=${orderIds.join(",")}&cart=1`;
    } catch (error) {
        console.error(error);
        // 登録済みの注文を再登録しないよう、成功分は決済画面へ進む
        if (orderIds.length > 0) {
            alert("一部の注文だけ登録されました。登録済みの注文を決済します。");
            window.location.href = `payment.html?orders=${orderIds.join(",")}&cart=1`;
        } else {
            alert("注文処理中にエラーが発生しました。");
            checkoutBtn.disabled = false;
        }
    }
}

function escapeHtml(str) {
    if (!str) return "";
    return str.replace(/[&<>"']/g, function(m) {
        return {
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#39;'
        }[m];
    });
}