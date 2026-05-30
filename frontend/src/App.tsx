import { useEffect, useRef, useState } from 'react'
import splashSvg from './assets/login.svg'
import loginSvg from './assets/login1.svg'
import registerSvg from './assets/register.svg'
import gameBgSvg from './assets/background.svg'
import tutorialSvg from './assets/tutorial.svg'
import settingsSvg from './assets/settings.svg'

type View = 'splash' | 'login' | 'register' | 'game'

const TOKEN_KEY = 'token'
const USERNAME_KEY = 'username'

async function login(username: string, password: string) {
  const res = await fetch('/v1/users/login', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    // Backend contract is { phone, password }. Treat username as phone here.
    body: JSON.stringify({ name: username, password }),
  })

  if (!res.ok) throw new Error('login_failed')
  const data = (await res.json()) as { token?: string }
  if (!data.token) throw new Error('missing_token')
  return data.token
}

async function startGame(name: string, token: string) {
  const res = await fetch('/v1/game/start', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ name }),
  })

  if (!res.ok) throw new Error('start_failed')
  const data = (await res.json()) as { matchID?: string }
  if (!data.matchID) throw new Error('missing_match_id')
  return data.matchID
}

async function getMatchStatus(matchID: string, token: string) {
  const res = await fetch(`/v1/game/status/${encodeURIComponent(matchID)}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!res.ok) throw new Error('status_failed')
  return (await res.json()) as { status?: string; roomID?: string }
}

function toWsUrl(roomID: string, token: string) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const qs = new URLSearchParams({ token })
  return `${protocol}//${window.location.host}/ws/room/${encodeURIComponent(roomID)}?${qs.toString()}`
}

async function register(username: string, password: string) {
  const res = await fetch('/v1/users/register', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    body: JSON.stringify({ name: username, password }),
  })

  if (!res.ok) throw new Error('register_failed')
}

async function verifyToken(token: string) {
  try {
    const res = await fetch('/v1/users/verify', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
      },
      body: JSON.stringify({ token }),
    })

    if (!res.ok) return false
    const data = (await res.json()) as { valid?: boolean }
    return data.valid === true
  } catch {
    return false
  }
}

function Game(props: { toast: (message: string) => void; blocked: boolean }) {
  const [pressed, setPressed] = useState(false)
  const [tutorialPressed, setTutorialPressed] = useState(false)
  const [settingsPressed, setSettingsPressed] = useState(false)
  const [matching, setMatching] = useState(false)
  const [dots, setDots] = useState(1)
  const dotsTimerRef = useRef<number | null>(null)

  const pollTimerRef = useRef<number | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [connectedRoomID, setConnectedRoomID] = useState<string | null>(null)

  useEffect(() => {
    return () => {
      if (pollTimerRef.current != null) window.clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
      if (wsRef.current) wsRef.current.close()
      wsRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!matching) return
    dotsTimerRef.current = window.setInterval(() => {
      setDots((d) => (d % 3) + 1)
    }, 450)
    return () => {
      if (dotsTimerRef.current != null) window.clearInterval(dotsTimerRef.current)
      dotsTimerRef.current = null
    }
  }, [matching])

  const onPressStart = () => {
    if (props.blocked || matching) return
    setPressed(true)
  }

  const onPressEnd = () => {
    setPressed(false)
  }

  const onTutorialPressStart = () => {
    if (props.blocked) return
    setTutorialPressed(true)
  }

  const onTutorialPressEnd = () => {
    setTutorialPressed(false)
  }

  const onSettingsPressStart = () => {
    if (props.blocked) return
    setSettingsPressed(true)
  }

  const onSettingsPressEnd = () => {
    setSettingsPressed(false)
  }

  const submit = async () => {
    if (props.blocked || matching || connectedRoomID) return

    const token = localStorage.getItem(TOKEN_KEY)
    const name = localStorage.getItem(USERNAME_KEY)

    if (!token) {
      props.toast('未登录')
      return
    }
    if (!name) {
      props.toast('缺少用户名')
      return
    }

    setMatching(true)
    setDots(1)
    try {
      // Backend currently expects {matchid} to be the username (not the numeric matchID).
      await startGame(name, token)

      if (pollTimerRef.current != null) window.clearInterval(pollTimerRef.current)
      pollTimerRef.current = window.setInterval(async () => {
        try {
          const s = await getMatchStatus(name, token)
          if (s.status !== 'IN_GAME') return
          if (!s.roomID) throw new Error('missing_room_id')

          window.clearInterval(pollTimerRef.current!)
          pollTimerRef.current = null
          setMatching(false)

          const ws = new WebSocket(toWsUrl(s.roomID, token))
          wsRef.current = ws
          setConnectedRoomID(s.roomID)

          ws.addEventListener('message', (e) => {
            // Keep minimal: log for now.
            console.log('[ws]', e.data)
          })
          ws.addEventListener('close', () => {
            wsRef.current = null
            setConnectedRoomID(null)
          })
        } catch {
          if (pollTimerRef.current != null) window.clearInterval(pollTimerRef.current)
          pollTimerRef.current = null
          setMatching(false)
          props.toast('匹配失败')
        }
      }, 1000)
    } catch {
      setMatching(false)
      props.toast('匹配失败')
    }
  }

  return (
    <section className="game" aria-label="game">
      <img className="gameBg" src={gameBgSvg} alt="" aria-hidden="true" />
      <div className="gameBottom" aria-hidden="true" />
      <button
        className={pressed ? 'enterBtn enterBtn--pressed' : 'enterBtn'}
        type="button"
        onPointerDown={onPressStart}
        onPointerUp={onPressEnd}
        onPointerCancel={onPressEnd}
        onPointerLeave={onPressEnd}
        onClick={submit}
        disabled={props.blocked || matching || !!connectedRoomID}
      >
        <span className="enterBtnInner">
          {matching ? `Matching${'.'.repeat(dots)}` : 'Enter'}
        </span>
      </button>

      <button
        className={tutorialPressed ? 'tutorialBtn tutorialBtn--pressed' : 'tutorialBtn'}
        type="button"
        onPointerDown={onTutorialPressStart}
        onPointerUp={onTutorialPressEnd}
        onPointerCancel={onTutorialPressEnd}
        onPointerLeave={onTutorialPressEnd}
        disabled={props.blocked}
        aria-label="tutorial"
      >
        <img className="tutorialIcon" src={tutorialSvg} alt="" aria-hidden="true" />
      </button>

      <button
        className={settingsPressed ? 'settingsBtn settingsBtn--pressed' : 'settingsBtn'}
        type="button"
        onPointerDown={onSettingsPressStart}
        onPointerUp={onSettingsPressEnd}
        onPointerCancel={onSettingsPressEnd}
        onPointerLeave={onSettingsPressEnd}
        disabled={props.blocked}
        aria-label="settings"
      >
        <img className="settingsIcon" src={settingsSvg} alt="" aria-hidden="true" />
      </button>
    </section>
  )
}

function Toast(props: { message: string }) {
  return (
    <div className="toast" role="status" aria-live="polite">
      {props.message}
    </div>
  )
}

function Fade(props: { on: boolean }) {
  return <div className={props.on ? 'fade fade--on' : 'fade'} aria-hidden="true" />
}

function Register(props: {
  onRegistered: () => void
  onBack: () => void
  toast: (message: string) => void
  blocked: boolean
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [password2, setPassword2] = useState('')
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (pending || props.blocked) return
      props.onBack()
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [pending, props.blocked, props.onBack])

  const submit = async () => {
    if (pending || props.blocked) return
    if (!username.trim() || !password) {
      props.toast('请填写用户名和密码')
      return
    }
    if (password !== password2) {
      props.toast('两次密码不一致')
      return
    }

    setPending(true)
    try {
      await register(username.trim(), password)
      props.onRegistered()
    } catch {
      props.toast('注册失败')
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="page" aria-label="register">
      <div className="frame">
        <div className="canvas">
          <img className="bg" src={registerSvg} alt="" aria-hidden="true" />

          <div className="registerOverlay">
            <input
              className="loginField registerFieldUser"
              type="text"
              placeholder="用户名"
              autoComplete="username"
              disabled={pending || props.blocked}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <input
              className="loginField registerFieldPass"
              type="password"
              placeholder="密码"
              autoComplete="new-password"
              disabled={pending || props.blocked}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <input
              className="loginField registerFieldPass2"
              type="password"
              placeholder="确认密码"
              autoComplete="new-password"
              disabled={pending || props.blocked}
              value={password2}
              onChange={(e) => setPassword2(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') submit()
              }}
            />

            <button
              className="registerSubmit"
              type="button"
              onClick={submit}
              disabled={pending || props.blocked}
            >
              注册
            </button>
          </div>
        </div>
      </div>
    </main>
  )
}

function Login(props: {
  onLoggedIn: () => void
  onRegister: () => void
  onBack: () => void
  toast: (message: string) => void
  blocked: boolean
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (pending || props.blocked) return
      props.onBack()
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [pending, props.blocked, props.onBack])

  const submit = async () => {
    if (pending || props.blocked) return
    setPending(true)
    try {
      const token = await login(username.trim(), password)
      localStorage.setItem(TOKEN_KEY, token)
      localStorage.setItem(USERNAME_KEY, username.trim())
      props.onLoggedIn()
    } catch {
      props.toast('登录失败')
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="page" aria-label="login">
      <div className="frame">
        <div className="canvas">
          <img className="bg" src={loginSvg} alt="" aria-hidden="true" />

          <input
            className="loginField loginFieldUser"
            type="text"
            placeholder="用户名"
            autoComplete="username"
            inputMode="text"
            disabled={pending || props.blocked}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <input
            className="loginField loginFieldPass"
            type="password"
            placeholder="密码"
            autoComplete="current-password"
            disabled={pending || props.blocked}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />

          <button className="loginBtn" type="button" onClick={submit} disabled={pending || props.blocked}>
            登录
          </button>

          <button
            className="registerBtn"
            type="button"
            onClick={props.onRegister}
            disabled={pending || props.blocked}
            aria-label="register"
          />
        </div>
      </div>
    </main>
  )
}

export default function App() {
  const [view, setView] = useState<View>('splash')
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const toastTimerRef = useRef<number | null>(null)

  const [transitionOn, setTransitionOn] = useState(false)
  const [transitionBusy, setTransitionBusy] = useState(false)
  const transitionTimerRef = useRef<number | null>(null)
  const transitionBusyRef = useRef(false)

  const toast = (message: string) => {
    setToastMessage(message)
    if (toastTimerRef.current != null) window.clearTimeout(toastTimerRef.current)
    toastTimerRef.current = window.setTimeout(() => setToastMessage(null), 2200)
  }

  useEffect(() => {
    return () => {
      if (toastTimerRef.current != null) window.clearTimeout(toastTimerRef.current)
      if (transitionTimerRef.current != null) window.clearTimeout(transitionTimerRef.current)
    }
  }, [])

  const transitionTo = (next: View) => {
    if (transitionBusyRef.current) return
    if (next === view) return

    transitionBusyRef.current = true
    setTransitionBusy(true)
    setTransitionOn(true)

    // Fade to black.
    transitionTimerRef.current = window.setTimeout(() => {
      setView(next)

      // Next frame: start fading back in.
      requestAnimationFrame(() => {
        setTransitionOn(false)
        transitionTimerRef.current = window.setTimeout(() => {
          transitionBusyRef.current = false
          setTransitionBusy(false)
        }, 500)
      })
    }, 500)
  }

  useEffect(() => {
    const start = async () => {
      if (view !== 'splash') return
      if (transitionBusyRef.current) return

      const token = localStorage.getItem(TOKEN_KEY)
      if (!token) {
        transitionTo('login')
        return
      }

      const valid = await verifyToken(token)
      if (valid) {
        transitionTo('game')
      } else {
        localStorage.removeItem(TOKEN_KEY)
        transitionTo('login')
      }
    }

    const onKeyDown = (e: KeyboardEvent) => {
      // Backdoor: go straight to game from splash.
      if (view === 'splash' && e.key === 'Backspace') {
        transitionTo('game')
        return
      }
      void start()
    }

    const onPointerDown = () => {
      void start()
    }

    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('pointerdown', onPointerDown, { passive: true })
    // Fallback for environments without Pointer Events.
    window.addEventListener('mousedown', onPointerDown, { passive: true })
    window.addEventListener('touchstart', onPointerDown, { passive: true })
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('mousedown', onPointerDown)
      window.removeEventListener('touchstart', onPointerDown)
    }
  }, [view])

  const blocked = transitionBusy

  if (view === 'game') {
    return (
      <>
        <Game toast={toast} blocked={blocked} />
        <Fade on={transitionOn} />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  if (view === 'login') {
    return (
      <>
        <Login
          onLoggedIn={() => transitionTo('game')}
          onRegister={() => transitionTo('register')}
          onBack={() => transitionTo('splash')}
          toast={toast}
          blocked={blocked}
        />
        <Fade on={transitionOn} />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  if (view === 'register') {
    return (
      <>
        <Register
          onRegistered={() => transitionTo('login')}
          onBack={() => transitionTo('login')}
          toast={toast}
          blocked={blocked}
        />
        <Fade on={transitionOn} />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  return (
    <>
      <main className="page">
        <img
          className="art"
          src={view === 'splash' ? splashSvg : loginSvg}
          alt=""
          aria-hidden="true"
        />
      </main>
      <Fade on={transitionOn} />
      {toastMessage && <Toast message={toastMessage} />}
    </>
  )
}
