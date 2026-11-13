
let currentProduct = null;

document.addEventListener("DOMContentLoaded", () => {
    const urlParams = new URLSearchParams(window.location.search);
    const productId = urlParams.get("id");

    if (!productId) {
        alert("商品IDが指定されていません。");
        window.location.href = "index.html";
        return;
    }

    // 商品情報の取得と表示
    fetch(`/api/product?id=${productId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error("商品情報の取得に失敗しました。");
            }
            return response.json();
        })
        .then(product => {
            currentProduct = product;

            // 第22回追加：商品画像の表示
            const image = document.getElementById("product-image");

            if (image) {
                if (product.image_url) {
                    image.src = product.image_url;
                    image.hidden = false;
                } else {
                    image.removeAttribute("src");
                    image.hidden = true;
                }
            }

            // 商品名・価格の表示
            document.getElementById("product-name").innerText = product.name;
            document.getElementById("product-price").innerText =
                `¥${product.price.toLocaleString()}`;

            // 既存の「注文する」ボタン
            document.getElementById("order-btn").onclick =
                () => orderDetailProduct(product.id);

            // 第18回追加：「カートに入れる」ボタン
            document.getElementById("add-to-cart-btn").onclick =
                () => addToCart();
        })
        .catch(error => {
            console.error(error);
            alert("商品の読み込みに失敗しました。");
        });
});

// カートに追加する処理
function addToCart() {
    if (!currentProduct) return;

    const qtyInput = document.getElementById("quantity");
    const quantity = parseInt(qtyInput.value, 10);

    if (isNaN(quantity) || quantity <= 0) {
        alert("正当な数量を入力してください。");
        return;
    }

    // localStorageから既存カートの取得
    let cart = JSON.parse(localStorage.getItem("cart")) || [];

    // すでに同じ商品がカート内にあるか確認
    const existingIndex = cart.findIndex(
        item => item.id === currentProduct.id
    );

    if (existingIndex !== -1) {
        // 同じ商品が存在する場合は数量を加算
        cart[existingIndex].quantity += quantity;
    } else {
        // 新規商品の場合は追加
        cart.push({
            id: currentProduct.id,
            name: currentProduct.name,
            price: currentProduct.price,
            quantity: quantity
        });
    }

    // localStorageに保存
    localStorage.setItem("cart", JSON.stringify(cart));

    // メッセージ表示
    const msgDiv = document.getElementById("message");
    msgDiv.innerText = "カートに追加しました";
    msgDiv.style.display = "block";

    setTimeout(() => {
        msgDiv.style.display = "none";
    }, 3000);
}

// 既存の注文処理
function orderDetailProduct(productId) {
    const qtyInput = document.getElementById("quantity");
    const quantity = parseInt(qtyInput.value, 10);

    if (isNaN(quantity) || quantity <= 0) {
        alert("正当な数量を入力してください。");
        return;
    }

    const payload = {
        product_id: productId,
        quantity: quantity
    };

    fetch("/api/orders", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify(payload)
    })
    .then(response => {
        if (!response.ok) {
            throw new Error("注文処理に失敗しました。");
        }
        return response.json();
    })
    .then(data => {
        if (!Number.isSafeInteger(data.id) || data.id <= 0) {
            throw new Error("注文IDを取得できませんでした。");
        }

        window.location.href = `payment.html?orders=${data.id}`;
    })
    .catch(error => {
        console.error(error);
        alert("注文に失敗しました。");
    });
}
