import { createRoot } from 'react-dom/client'
import '../../../../design/tokens.css'
import './styles.css'
import App from './App'

const container = document.getElementById('root')
if (!container) throw new Error('Launcher root element is missing.')

createRoot(container).render(<App />)
