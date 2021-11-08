import React, { useEffect, useState, useCallback } from 'react';

export default function Chat() {
    const [chatState, setChatState] = useState({username: "", text: ""})
    const [ws] = useState(new WebSocket("ws://localhost:8080"))
    const [messages, setMessages] = useState([])

    const joinChannel = useCallback((channel) => {
        const obj = {"type": "join_channel", "payload": {"channel": channel}}
        ws.send(JSON.stringify(obj))
    }, [ws])

    const channelMessage = useCallback(() => {
        const obj = {"type": "channel_message", "payload": {"channel": "2", "message": chatState.text}}
        ws.send(JSON.stringify(obj))
    }, [ws, chatState])

    const handleMessage = useCallback(msg => {
        if (msg.data == null) {
            return
        }

        const wsMessage = JSON.parse(msg.data)
        switch (wsMessage.type) {
            case "joined_channel": {
                if (chatState.username === "") {
                    setChatState({username: wsMessage.payload.username})
                }
                break;
            }
            case "channel_message": {
                setMessages([...messages, wsMessage])
                break;
            }
            default:
                console.log(wsMessage)
                break
        }
    }, [chatState, messages])

    useEffect(() => {
        ws.onopen = () => joinChannel("2")
        ws.onmessage = handleMessage
    }, [ws, handleMessage, joinChannel])

    const sendMessage = () => {
        channelMessage()
        console.log(chatState)
    }

    return (
        <div>
            <ul>
                {messages.map((message, index) =>
                    <li key={index}>
                        <b>{message.payload.from}</b>: <em>{message.payload.message}</em>
                    </li>
                )}
            </ul>
            <input type="text" id="message" onChange={e => setChatState({...chatState, text: e.target.value})}/>
            <button onClick={sendMessage}>send</button>
        </div>
    )
}
