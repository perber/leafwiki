import { Input } from '@/components/ui/input'

type FormInputProps = {
  label?: string
  name?: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  testid?: string
  error?: string
  type?: string
  autoComplete?: string
  autoFocus?: boolean
  readOnly?: boolean
  allowedHotkeys?: string
}

export function FormInput({
  label,
  name,
  value,
  autoComplete,
  autoFocus,
  onChange,
  testid,
  placeholder,
  error,
  type = 'text',
  readOnly = false,
  allowedHotkeys,
}: FormInputProps) {
  return (
    <div className="form-input">
      {label && <label className="form-input__label">{label}</label>}
      <Input
        autoFocus={autoFocus || false}
        name={name}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete={autoComplete}
        readOnly={readOnly}
        className={error ? 'form-input__input-error' : ''}
        data-testid={testid}
        data-allow-hotkeys={allowedHotkeys}
      />
      {error && <p className="form-input__error">{error}</p>}
    </div>
  )
}
