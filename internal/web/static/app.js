document.addEventListener('DOMContentLoaded', () => {
    const cardContent = document.getElementById('card-content');
    const nextButton = document.getElementById('next-button');

    async function getNextCard() {
        try {
            const response = await fetch('/api/next');
            if (response.status === 204) {
                cardContent.innerHTML = '<p>No cards due for review! Enjoy your break.</p>';
                nextButton.style.display = 'none';
                return;
            }
            const card = await response.json();
            cardContent.innerHTML = `
                <p><strong>Question:</strong> ${card.question}</p>
                <button onclick="reviewCard('${card.hash}', 3)">Review (Good)</button>
            `;
        } catch (error) {
            console.error('Error fetching next card:', error);
            cardContent.innerHTML = '<p>Error loading card.</p>';
        }
    }

    window.reviewCard = async (hash, grade) => {
        try {
            await fetch('/api/review', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ card_hash: hash, grade: grade }),
            });
            getNextCard(); // Fetch the next card after reviewing
        } catch (error) {
            console.error('Error reviewing card:', error);
        }
    };

    nextButton.addEventListener('click', getNextCard);

    // Initial load
    getNextCard();
});