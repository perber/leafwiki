import { BrowserRouter, Route, Routes } from "react-router-dom"
import './App.css'
import PageViewer from './features/page/PageViewer'
import AppLayout from './layout/AppLayout'

function App() {
  return (
    <BrowserRouter>
      <AppLayout>
        <Routes>
          <Route path="*" element={<PageViewer />} />
        </Routes>
      </AppLayout>
    </BrowserRouter>
  )
}

export default App
