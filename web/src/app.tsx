import { BrowserRouter } from 'react-router-dom'
import { RouterView } from './router'
import { AuthBootstrap } from './components/AuthBootstrap'

export function App() {
  return (
    <BrowserRouter>
      <AuthBootstrap>
        <RouterView />
      </AuthBootstrap>
    </BrowserRouter>
  )
}
