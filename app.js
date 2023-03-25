let uuid;

const socket = new WebSocket('ws://localhost:8080/v1/ws');
socket.onopen = () => {
    console.log('WebSocket connection established.');
};

socket.addEventListener('open', (event) => {
    const connectionHeader = 'Upgrade';
    const httpRequest = socket._socket._request;
    httpRequest.setRequestHeader('Connection', connectionHeader);
});

socket.addEventListener('message', (event) => {
    console.log('WebSocket message received:', event.data);
    uuid = event.data;
});

socket.addEventListener('close', (event) => {
    console.log('WebSocket connection closed:', event.code, event.reason);
});

socket.addEventListener('error', (event) => {
    console.log('WebSocket error:', event);
});

// Add event listener to HTTP button
const httpButton = document.getElementById("http-button");
httpButton.addEventListener("click", () => {
    const url = "http://localhost:8080/v1/match";
    const data = new FormData();
    data.append("price", "10.0");
    data.append("uuid", uuid);
    const options = {
        method: "POST",
        body: data
    };
    fetch(url, options)
        .then(response => {
            if (response.ok) {
                console.log("Request sent successfully.");
            } else {
                console.error("Failed to send request:", response.status);
            }
        })
        .catch(error => {
            console.error("Failed to send request:", error);
        });
});
