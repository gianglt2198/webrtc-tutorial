let ws = null;
let localStream = null;
let roomId = null;
let peerId = null;

const iceServers = {
    iceServers: [{
        urls: ["stun:stun.l.google.com:19302"]
    }],
    iceCandidatePoolSize: 10,
}
let peers = {}; // Active peer connections  
let remoteStreams = {}; // Remote media streams  

// DOM Elements  
const localVideo = document.getElementById('localVideo');
const remoteVideosContainer = document.getElementById('remoteVideosContainer');

let isJoining = false;
let joinButton = null;


function uuidv4() {
    return ([1e7] + -1e3 + -4e3 + -8e3 + -1e11).replace(/[018]/g, c =>
        (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
    );
}

function updateStatus(text) {
    document.getElementById('status').textContent = text;
}

// --------------------------  
// Core Functionality  
// --------------------------  

async function joinRoom() {
    if (isJoining) return;
    isJoining = true;

    joinButton = document.getElementById('joinButton');
    const originalButtonText = joinButton.textContent;

    try {
        // Disable UI during join process  
        joinButton.disabled = true;
        joinButton.textContent = 'Joining...';

        if (!peerId) peerId = uuidv4();

        const newRoomId = document.getElementById("roomId").value || uuidv4();
        if (roomId && roomId !== newRoomId) {
            await leaveRoom();
        }
        roomId = newRoomId;

        if (!ws || ws.readyState !== WebSocket.OPEN) {
            setupWebSocket();
        } else {
            await new Promise(resolve => setTimeout(resolve, 500));
        }

        if (!localStream) {
            localStream = await navigator.mediaDevices.getUserMedia({
                video: true,
                audio: true
            });
            localVideo.srcObject = localStream;
        }

        updateStatus(`Joining room ${roomId}...`);
        await new Promise(resolve => setTimeout(resolve, 1000));
    } catch (error) {
        console.error('Media device error:', error);
        updateStatus('Failed to access media devices');
    } finally {
        isJoining = false;
        if (joinButton) {
            joinButton.disabled = false;
            joinButton.textContent = originalButtonText;
        }
    }
}

async function leaveRoom() {
    //     Object.values(peers).forEach(pc => pc.close());
    //     peers = {};

    //     localStream?.getTracks().forEach(track => track.stop());
    //     Object.values(remoteStreams).forEach(stream =>
    //         stream.getTracks().forEach(track => track.stop())
    //     );

    //     if (ws) {
    //         ws.send(JSON.stringify({ type: "leave", room_id: roomId }));
    //         ws.close();
    //     }

    //     remoteVideosContainer.innerHTML = '';
    //     updateStatus(`Left room`);
    const leaveActions = [
        () => {
            Object.values(peers).forEach(pc => pc.close());
            peers = {};
        },
        () => {
            if (localStream) {
                localStream.getTracks().forEach(track => track.stop());
                localStream = null;
            }
            Object.values(remoteStreams).forEach(stream =>
                stream.getTracks().forEach(track => track.stop())
            );
            remoteStreams = null;
        },
        () => {
            if (ws) {
                ws.send(JSON.stringify({
                    type: "leave",
                    room_id: roomId,
                    sender_id: peerId
                }));
                ws.close();
                ws = null;
            }
        }
    ];

    await Promise.allSettled(leaveActions.map(action => action()));
    peerId = null;
    roomId = null;
    remoteVideosContainer.innerHTML = '';
    updateStatus(`Left room`);
}

// --------------------------  
// WebSocket & Signaling  
// --------------------------  

function setupWebSocket() {
    if (ws) {
        ws.onmessage = null;
        ws.onclose = null;
        ws.close();
    }

    ws = new WebSocket(`ws:${window.location.host}/ws`);

    ws.onopen = () => {
        const joinPayload = {
            type: "join",
            room_id: roomId,
            sender_id: peerId
        };
        setTimeout(() => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify(joinPayload));
            }
        }, 300);
    };

    ws.onmessage = async (event) => {
        const msg = JSON.parse(event.data);
        switch (msg.type) {
            case 'joined':
                handleJoined(msg);
                break;
            case 'offer':
                handleOffer(msg);
                break;
            case 'answer':
                handleAnswer(msg);
                break;
            case 'candidate':
                handleCandidate(msg);
                break
            case 'new-participant':
                handleNewParticipant(msg);
                break
            case 'participant-left':
                handleParticipantLeft(msg);
                break;
            case 'participants-list':
                handleParticipantsList(msg);
                break;
        }
    };
}

// --------------------------  
// Peer Connection Management  
// --------------------------  

async function createPeerConnection(targetPeerId) {
    const pc = new RTCPeerConnection(iceServers);
    peers[targetPeerId] = pc;

    // Add local tracks  
    localStream.getTracks().forEach(track =>
        pc.addTrack(track, localStream)
    );

    // ICE Candidate handling  
    pc.onicecandidate = ({ candidate }) => {
        if (candidate) {
            ws.send(JSON.stringify({
                type: "candidate",
                room_id: roomId,
                sender_id: peerId,
                target_id: targetPeerId,
                payload: JSON.stringify(candidate.toJSON())
            }));
        }
    };

    // Remote stream handling  
    pc.ontrack = (event) => {
        const stream = event.streams[0];
        handleRemoteStream(targetPeerId, stream);
    };

    // Connection state monitoring  
    pc.onconnectionstatechange = () => {
        console.log(`${targetPeerId} connection state:`, pc.connectionState);
        if (pc.connectionState === 'disconnected') {
            handleParticipantLeft({ sender_id: targetPeerId });
        }
    };

    return pc;
}

// --------------------------  
// Message Handlers  
// --------------------------  

async function handleJoined(msg) {
    peerId = msg.sender_id;

    updateStatus(`Joined room ${roomId} as ${peerId}`);

    // Request existing participants from server  
    ws.send(JSON.stringify({
        type: "get-participants",
        room_id: roomId,
        sender_id: peerId
    }));
}

async function handleParticipantsList(msg) {
    const existingPeers = msg.peers || [];
    updateStatus(`Found ${existingPeers.length} existing participants`);

    // existingPeers.forEach(async (existingPeerId) => {
    //     if (existingPeerId !== peerId && !peers[existingPeerId]) {
    //         await handleNewParticipant({
    //             type: 'new-participant',
    //             sender_id: existingPeerId
    //         });
    //     }
    // });
}

async function handleNewParticipant(msg) {
    const newPeerId = msg.sender_id;
    if (newPeerId === peerId || peers[newPeerId]) return;

    // Create a new RTCPeerConnection for this participant  
    const pc = new RTCPeerConnection(iceServers);
    peers[newPeerId] = pc;

    localStream.getTracks().forEach(track => {
        pc.addTrack(track, localStream);
    });

    // Handle remote stream for this peer  
    pc.ontrack = (event) => {
        handleRemoteStream(newPeerId, event.streams[0]);
    };

    // Handle ICE candidates  
    pc.onicecandidate = ({ candidate }) => {
        if (candidate) {
            ws.send(JSON.stringify({
                type: "candidate",
                room_id: roomId,
                sender_id: peerId,
                target_id: newPeerId,
                payload: JSON.stringify(candidate.toJSON()),
            }));
        }
    };

    // Send an offer to the new participant  
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    ws.send(JSON.stringify({
        type: "offer",
        room_id: roomId,
        sender_id: peerId,
        target_id: newPeerId,
        payload: JSON.stringify(offer),
    }));
}

async function handleOffer(msg) {
    const senderId = msg.sender_id;
    if (senderId === peerId) return;

    let pc = peers[senderId];
    if (!pc) pc = await createPeerConnection(senderId);

    const offer = JSON.parse(msg.payload);
    await pc.setRemoteDescription(new RTCSessionDescription(offer));

    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);

    ws.send(JSON.stringify({
        type: "answer",
        room_id: roomId,
        sender_id: peerId,
        target_id: senderId,
        payload: JSON.stringify(answer)
    }));
}

async function handleAnswer(msg) {
    const senderId = msg.sender_id;
    const pc = peers[senderId];
    if (!pc) return;

    const answer = JSON.parse(msg.payload);
    await pc.setRemoteDescription(new RTCSessionDescription(answer));
}

async function handleCandidate(msg) {
    const senderId = msg.sender_id;
    const pc = peers[senderId];
    if (!pc) return;

    const candidate = JSON.parse(msg.payload);
    await pc.addIceCandidate(new RTCIceCandidate(candidate));

    ws.send(JSON.stringify({
        type: "get-participants",
        room_id: roomId,
        sender_id: peerId
    }));
}

function handleParticipantLeft(msg) {
    const leftPeerId = msg.sender_id;

    // Cleanup peer connection  
    if (peers[leftPeerId]) {
        peers[leftPeerId].close();
        delete peers[leftPeerId];
    }

    // Remove video element  
    const videoElement = document.getElementById(leftPeerId);
    if (videoElement) videoElement.remove();

    // Cleanup stream  
    if (remoteStreams[leftPeerId]) {
        remoteStreams[leftPeerId].getTracks().forEach(track => track.stop());
        delete remoteStreams[leftPeerId];
    }

    ws.send(JSON.stringify({
        type: "get-participants",
        room_id: roomId,
        sender_id: peerId
    }));
}

// --------------------------  
// UI Helpers  
// --------------------------  

function handleRemoteStream(peerId, stream) {
    let videoElement = document.getElementById(peerId);

    if (!videoElement) {
        videoElement = document.createElement('video');
        videoElement.id = peerId;
        videoElement.autoplay = true;
        videoElement.playsInline = true;
        videoElement.classList.add('remote-video');
        remoteVideosContainer.appendChild(videoElement);
    }

    remoteStreams[peerId] = stream;
    videoElement.srcObject = stream;
}

function updateStatus(text) {
    const statusElement = document.getElementById('status');
    if (statusElement) statusElement.textContent = text;
}

function uuidv4() {
    return ([1e7] + -1e3 + -4e3 + -8e3 + -1e11).replace(/[018]/g, c =>
        (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
    );
}