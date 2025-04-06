import { Input } from '@/components/ui/input'

type FormInputProps = {
  label?: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  error?: string
  type?: string
  autoFocus?: boolean
}

export function FormInput({
  label,
  value,
  autoFocus,
  onChange,
  placeholder,
  error,
  type = 'text',
}: FormInputProps) {
  return (
    <div className="space-y-1">
      {label && (
        <label className="block text-sm font-medium text-gray-700">
          {label}
        </label>
      )}
      <Input
        autoFocus={autoFocus || false}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={error ? 'border-red-500' : ''}
      />
      {error && <p className="text-sm text-red-500">{error}</p>}
    </div>
  )
}
