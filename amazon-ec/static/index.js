document.addEventListener('DOMContentLoaded', () => {
    loadProducts();
});

async function loadProducts() {
    try {
        const response = await fetch('/api/products');
        if (!response.ok) {
            throw new Error('Failed to fetch products');
        }
        const products = await response.json();

        const productList = document.getElementById('product-list');
        productList.innerHTML = '';

        if (!products || products.length === 0) {
            productList.innerHTML = '<p>商品がありません。</p>';
            return;
        }

        products.forEach(product => {
            const card = document.createElement('div');
            card.className = 'product-card';
            card.innerHTML = `
                <img src="${product.image}" alt="${product.name}">
                <h3><a href="/static/detail.html?id=${product.id}">${product.name}</a></h3>
                <p class="price">¥${product.price.toLocaleString()}</p>
                <p class="rating">★ ${product.rating} (${product.reviews})</p>
                <div style="margin-top: 10px;">
                    <label>数量: <input type="number" id="qty-${product.id}" value="1" min="1" style="width: 50px;"></label>
                    <button class="btn" onclick="directBuy(${product.id})">注文する</button>
                </div>
            `;
            productList.appendChild(card);
        });
    } catch (error) {
        console.error('Error loading products:', error);
        document.getElementById('product-list').innerHTML = '<p>商品情報の取得に失敗しました。</p>';
    }
}

async function directBuy(productId) {
    const qtyInput = document.getElementById(`qty-${productId}`);
    const quantity = qtyInput ? (parseInt(qtyInput.value, 10) || 1) : 1;

    try {
        const response = await fetch('/api/orders', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                product_id: parseInt(productId, 10),
                quantity: quantity
            })
        });

        if (response.status === 401) {
            alert('注文するにはログインが必要です');
            return;
        }

        if (response.ok) {
            window.location.href = '/static/payment.html';
        } else {
            alert('注文処理に失敗しました');
        }
    } catch (error) {
        console.error('Error in direct buy:', error);
        alert('エラーが発生しました');
    }
}